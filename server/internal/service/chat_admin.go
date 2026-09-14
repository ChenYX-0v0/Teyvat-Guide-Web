package service

// 后台对话记录查看（内网只读）
// 契约：admin/06-对话记录查看设计.md §4
//
// 只读红线：不提供修改/删除消息的能力（对话记录属于审计数据）。

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
)

// ErrSessionNotFound 会话不存在（Handler 映射为 404）
var ErrSessionNotFound = errors.New("会话不存在")

// ErrChatQueryInvalid 查询参数有误（Handler 映射为 400）
var ErrChatQueryInvalid = errors.New("查询参数有误")

const (
	chatMessageDefaultLimit = 200
	chatMessageMaxLimit     = 500
	lastQuestionMaxRunes    = 60 // 列表里的提问摘要长度
)

// ChatAdminService 对话记录的只读查询
type ChatAdminService struct {
	repo *repository.ChatRepository
}

func NewChatAdminService() *ChatAdminService {
	return &ChatAdminService{repo: repository.NewChatRepository()}
}

// ListSessions 会话列表（分页）
func (s *ChatAdminService) ListSessions(q dto.AdminChatQuery) (*dto.AdminChatSessionList, error) {
	page, pageSize := normalizePaging(q.Page, q.PageSize)

	from, err := parseRFC3339(q.From)
	if err != nil {
		return nil, fmt.Errorf("%w: from %s", ErrChatQueryInvalid, err)
	}
	to, err := parseRFC3339(q.To)
	if err != nil {
		return nil, fmt.Errorf("%w: to %s", ErrChatQueryInvalid, err)
	}

	rows, total, err := s.repo.ListAdminSessions(repository.AdminSessionFilter{
		Keyword:      strings.TrimSpace(q.Keyword),
		UserID:       q.UserID,
		UID:          strings.TrimSpace(q.UID),
		OnlyDegraded: q.OnlyDegraded,
		OnlyDeleted:  q.OnlyDeleted,
		OnlyOffTopic: q.OnlyOffTopic,
		From:         from,
		To:           to,
		Offset:       (page - 1) * pageSize,
		Limit:        pageSize,
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	lastQuestions, qErr := s.repo.ListLastQuestions(ids)
	if qErr != nil {
		// 摘要取不到不影响列表可用性（该列为空）
		logger.L().Warnw("load last questions failed", "err", qErr)
	}

	items := make([]dto.AdminChatSessionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.AdminChatSessionItem{
			SessionID:         row.ID,
			Title:             row.Title,
			UserID:            row.UserID,
			Username:          optionalString(row.Username),
			DisplayName:       optionalString(row.DisplayName),
			GenshinUID:        optionalString(row.GenshinUID),
			MessageCount:      row.MessageCount,
			Tokens:            row.Tokens,
			DegradedCount:     row.DegradedCount,
			HasPlayerContext:  row.HasPlayerContext,
			HasAssetReference: row.HasAssetReference,
			LastQuestion:      optionalString(summarize(lastQuestions[row.ID], lastQuestionMaxRunes)),
			CreatedAt:         row.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         row.UpdatedAt.Format(time.RFC3339),
			DeletedAt:         formatOptionalTime(row.DeletedAt),
			OffTopicCount:     row.OffTopicCount,
		})
	}

	return &dto.AdminChatSessionList{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Messages 会话明细（游标分页：afterId 之后的消息，按 id 升序）
func (s *ChatAdminService) Messages(sessionID string, afterID uint, limit int) (*dto.AdminChatMessages, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: sessionId 不能为空", ErrChatQueryInvalid)
	}
	if limit <= 0 || limit > chatMessageMaxLimit {
		limit = chatMessageDefaultLimit
	}

	session, err := s.repo.FindSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	// 多取一条用于判断 hasMore（避免额外 COUNT）
	list, err := s.repo.ListMessagesAfter(sessionID, afterID, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(list) > limit
	if hasMore {
		list = list[:limit]
	}

	items := make([]dto.AdminChatMessageItem, 0, len(list))
	for _, m := range list {
		items = append(items, dto.AdminChatMessageItem{
			ID:           m.ID,
			Role:         m.Role,
			Content:      m.Content,
			Tokens:       m.Tokens,
			TokensInput:  m.TokensInput,
			TokensOutput: m.TokensOutput,
			TokensCached: m.TokensCached,
			Model:        m.Model,
			Profile:      m.Profile,
			Degraded:     m.Degraded,
			ElapsedMs:    m.ElapsedMs,
			CreatedAt:    m.CreatedAt.Format(time.RFC3339),
			// 只有提问消息带判定；OffTopic 为 nil 表示未判定（前端不显示 Tag）
			OffTopic:       m.OffTopic,
			OffTopicReason: m.OffTopicReason,
		})
	}

	return &dto.AdminChatMessages{
		SessionID: session.ID,
		UserID:    session.UserID,
		Messages:  items,
		HasMore:   hasMore,
	}, nil
}

// normalizePaging 分页参数归一（与用户列表一致：pageSize 上限 100）
func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// parseRFC3339 解析可选时间参数；空串表示不过滤
func parseRFC3339(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, errors.New("时间格式应为 RFC3339")
	}
	return &t, nil
}

// optionalString 空串返回 nil（JSON 输出 null，前端区分"匿名/未绑定"）
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// formatOptionalTime 可空时间 → RFC3339 字符串（nil 输出 null，前端据此区分"未删除"）
func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// summarize 按字符数截断摘要
func summarize(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
