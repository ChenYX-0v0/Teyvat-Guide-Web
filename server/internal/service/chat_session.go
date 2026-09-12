package service

// C 端会话管理（需登录）：列表 / 明细 / 重命名
//
// 口径（2026-09-12 定，契约见 docs/04-API接口文档.md §3）：
//   - 只服务登录用户：列表仅返回 user_id = 本人的会话；匿名会话（user_id = 0）不进列表
//   - 消息一律从服务端拉（不在前端缓存），保证换设备 / 清缓存后一致
//   - 非本人或不存在的会话一律 ErrSessionNotFound（404）且不区分——否则接口可被用来探测会话是否存在
//   - 删除是**软删除**（置 chat_sessions.deleted_at）：C 端（列表/明细/改名/续聊）一律不可见，
//     数据保留供后台审计与成本统计；后台仍可查并标注「已删除」，且后台不提供删除入口

import (
	"fmt"
	"strings"
	"time"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/repository"
)

const (
	sessionMessageDefaultLimit = 100 // 打开会话时的一屏消息数
	sessionMessageMaxLimit     = 200
	sessionTitleMaxRunes       = 50 // 重命名后的标题长度上限（与建会话时的自动标题 20 字区分开）
)

// ChatSessionService 面向 C 端的会话管理
type ChatSessionService struct {
	repo *repository.ChatRepository
}

func NewChatSessionService() *ChatSessionService {
	return &ChatSessionService{repo: repository.NewChatRepository()}
}

// List 会话列表（分页，活跃时间倒序）
func (s *ChatSessionService) List(userID uint, page, pageSize int) (*dto.ChatSessionList, error) {
	if userID == 0 {
		// 路由已要求登录，这里是兜底：避免将来误把匿名请求接进来而返回"所有人的会话"
		return nil, fmt.Errorf("%w: 需要登录", ErrChatQueryInvalid)
	}
	page, pageSize = normalizePaging(page, pageSize)

	rows, total, err := s.repo.ListSessionsByUser(userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ChatSessionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.ChatSessionItem{
			SessionID:    row.ID,
			Title:        row.Title,
			MessageCount: row.MessageCount,
			CreatedAt:    row.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    row.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ChatSessionList{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Messages 会话明细：返回 id < beforeID 的最近 limit 条（beforeID = 0 表示取最新一屏），按 id 升序。
// 用 beforeID（而不是 afterID）是因为聊天是"往上翻历史"：打开会话先看最新，再往前加载更早的。
func (s *ChatSessionService) Messages(userID uint, sessionID string, beforeID uint, limit int) (*dto.ChatSessionMessages, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: sessionId 不能为空", ErrChatQueryInvalid)
	}
	if limit <= 0 || limit > sessionMessageMaxLimit {
		limit = sessionMessageDefaultLimit
	}

	session, err := s.findOwnedSession(userID, sessionID)
	if err != nil {
		return nil, err
	}

	// 多取一条用于判断 hasMore（是否还有更早的消息）
	list, err := s.repo.ListMessagesBefore(sessionID, beforeID, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(list) > limit
	if hasMore {
		// 结果按 id 升序，多出来的那条是最早的：它是"还有更早消息"的证据，不返回
		list = list[1:]
	}

	items := make([]dto.ChatSessionMessageItem, 0, len(list))
	for _, m := range list {
		items = append(items, dto.ChatSessionMessageItem{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ChatSessionMessages{
		SessionID: session.ID,
		Messages:  items,
		HasMore:   hasMore,
	}, nil
}

// Rename 重命名会话（仅本人）
func (s *ChatSessionService) Rename(userID uint, sessionID, title string) (*dto.RenamedSession, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: sessionId 不能为空", ErrChatQueryInvalid)
	}
	title, err := normalizeSessionTitle(title)
	if err != nil {
		return nil, err
	}

	ok, err := s.repo.RenameSession(sessionID, userID, title)
	if err != nil {
		return nil, err
	}
	if !ok {
		// 不存在或不属于调用者：统一 404（不暴露该会话是否存在）
		return nil, ErrSessionNotFound
	}

	return &dto.RenamedSession{SessionID: sessionID, Title: title}, nil
}

// Delete 删除会话（软删除，仅本人）：C 端从此不可见，数据保留供后台审计。
// 非本人 / 不存在 / 已删除，一律 ErrSessionNotFound（404：不暴露是否存在，也让重复删除自然幂等）。
func (s *ChatSessionService) Delete(userID uint, sessionID string) (*dto.DeletedSession, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: sessionId 不能为空", ErrChatQueryInvalid)
	}
	if userID == 0 {
		return nil, ErrSessionNotFound
	}

	ok, err := s.repo.SoftDeleteSession(sessionID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrSessionNotFound
	}

	return &dto.DeletedSession{SessionID: sessionID}, nil
}

// findOwnedSession 取会话并校验归属；userID = 0（匿名）一律视为不存在
// （匿名会话无法通过 C 端接口访问，避免"匿名 id 被贴过来就能读"）。
// 已软删除的会话对 C 端同样视为不存在（但后台仍可查，见 /internal/v1/admin/chat/*）。
func (s *ChatSessionService) findOwnedSession(userID uint, sessionID string) (*model.ChatSession, error) {
	if userID == 0 {
		return nil, ErrSessionNotFound
	}
	found, err := s.repo.FindSession(sessionID)
	if err != nil {
		return nil, err
	}
	if found == nil || found.UserID != userID || found.DeletedAt != nil {
		return nil, ErrSessionNotFound
	}
	return found, nil
}

// normalizeSessionTitle 归一化标题：去首尾空白；空或超长都视为参数错误
// （前端输入框已限制 maxLength，这里是服务端兜底，不做静默截断——静默改内容更难排查）
func normalizeSessionTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", fmt.Errorf("%w: 标题不能为空", ErrChatQueryInvalid)
	}
	if r := []rune(title); len(r) > sessionTitleMaxRunes {
		return "", fmt.Errorf("%w: 标题最长 %d 字", ErrChatQueryInvalid, sessionTitleMaxRunes)
	}
	return title, nil
}
