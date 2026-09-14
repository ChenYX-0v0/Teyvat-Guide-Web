package service

// 用户管理（供后台运维系统经内网接口调用）
// 契约：04-用户体系与鉴权边界设计 §4

import (
	"errors"
	"strings"
	"time"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
)

var ErrUserNotFound = errors.New("用户不存在")

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type UserAdminService struct {
	users *repository.UserRepository
	chats *repository.ChatRepository
}

func NewUserAdminService() *UserAdminService {
	return &UserAdminService{
		users: repository.NewUserRepository(),
		chats: repository.NewChatRepository(),
	}
}

// List 分页查询用户（keyword 匹配 用户名 / 昵称 / 游戏 UID），附带累计 token、当日用量与无关提问数。
// 用量与无关问题口径见 shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md §2.5 / §3.4。
func (s *UserAdminService) List(q dto.AdminUserQuery) (*dto.AdminUserList, error) {
	page, pageSize := normalizePaging(q.Page, q.PageSize)

	// 用量 / 无关问题这两类条件是**按消息聚合**出来的：先算出候选用户 ID 集合，
	// 再参与列表过滤，这样 total 与分页天然一致（nil = 不过滤，非 nil 空集 = 无命中）
	includeIDs, err := s.resolveUsageFilter(q)
	if err != nil {
		return nil, err
	}

	users, total, err := s.users.ListUsers(strings.TrimSpace(q.Keyword), includeIDs, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	tokenMap, err := s.chats.SumTokensByUsers(ids)
	if err != nil {
		// 聚合失败不影响列表可用性（仅 token 显示为 0）
		logger.L().Warnw("aggregate user tokens failed", "err", err)
	}
	from, to := usageWindow(q.UsageDate)
	usageMap, err := s.chats.SumUsageByUsersInRange(ids, from, to)
	if err != nil {
		logger.L().Warnw("aggregate user daily usage failed", "err", err)
	}
	offTopicMap, err := s.chats.SumOffTopicByUsers(ids)
	if err != nil {
		logger.L().Warnw("aggregate user off-topic failed", "err", err)
	}

	items := make([]dto.AdminUserItem, 0, len(users))
	for _, u := range users {
		usage := usageMap[u.ID]
		item := dto.AdminUserItem{
			ID:            u.ID,
			Username:      u.Username,
			DisplayName:   u.DisplayName,
			Status:        u.Status,
			CreatedAt:     u.CreatedAt.Format(time.RFC3339),
			TotalTokens:   tokenMap[u.ID],
			TodayTokens:   usage.Tokens,
			TodayInput:    usage.TokensInput,
			TodayOutput:   usage.TokensOutput,
			TodayCached:   usage.TokensCached,
			TodayRequests: usage.Requests,
			OffTopicCount: offTopicMap[u.ID],
		}
		if u.GenshinUID != "" {
			uid := u.GenshinUID
			item.GenshinUID = &uid
		}
		if u.LastLoginAt != nil {
			at := u.LastLoginAt.Format(time.RFC3339)
			item.LastLoginAt = &at
		}
		items = append(items, item)
	}

	return &dto.AdminUserList{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// resolveUsageFilter 把「当日用量 ≥ N」「有无关提问」翻译成候选用户 ID 集合。
// 返回 nil 表示不过滤；返回非 nil 空切片表示"没有任何用户命中"。
func (s *UserAdminService) resolveUsageFilter(q dto.AdminUserQuery) ([]uint, error) {
	var candidates []uint // nil = 还没有条件
	if q.MinDailyTokens > 0 {
		from, to := usageWindow(q.UsageDate)
		ids, err := s.chats.UserIDsWithTokensAtLeast(from, to, q.MinDailyTokens)
		if err != nil {
			return nil, err
		}
		candidates = intersect(candidates, ids)
	}
	if q.OnlyOffTopic {
		min := q.OffTopicMin
		if min < 1 {
			min = 1
		}
		ids, err := s.chats.UserIDsWithOffTopic(int64(min))
		if err != nil {
			return nil, err
		}
		candidates = intersect(candidates, ids)
	}
	return candidates, nil
}

// intersect 求交集：prev 为 nil 表示"还没有条件"，直接取 next。
// next 来自仓储，**一定是非 nil 切片**，因此空交集会保持非 nil → 上层判为"无命中"。
func intersect(prev, next []uint) []uint {
	if prev == nil {
		return next
	}
	set := make(map[uint]struct{}, len(prev))
	for _, id := range prev {
		set[id] = struct{}{}
	}
	out := make([]uint, 0)
	for _, id := range next {
		if _, ok := set[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

// usageWindow 把 usageDate（YYYY-MM-DD，**本地时区**）解析成 [当天 00:00, 次日 00:00)；
// 空或非法时用今天（与配额口径一致，见 shared-docs/05 §2.1）。
func usageWindow(date string) (time.Time, time.Time) {
	day := StartOfToday()
	if s := strings.TrimSpace(date); s != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
			day = parsed
		}
	}
	return day, day.AddDate(0, 0, 1)
}

// SetBanned 封禁 / 解封（幂等）。封禁后该用户既有 JWT 会在鉴权中间件被拒绝（见 IsBanned）。
func (s *UserAdminService) SetBanned(id uint, banned bool) error {
	n, err := s.users.CountByID(id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserNotFound
	}

	status := model.UserStatusActive
	if banned {
		status = model.UserStatusBanned
	}
	if err := s.users.UpdateStatus(id, status); err != nil {
		return err
	}
	logger.L().Infow("user status updated", "user", id, "status", status)
	return nil
}

// IsBanned 供鉴权中间件调用：封禁用户的既有 Token 立即失效
func (s *UserAdminService) IsBanned(userID uint) bool {
	u, err := s.users.FindByID(userID)
	if err != nil || u == nil {
		return false // 查询异常不阻断（避免误伤在线用户）
	}
	return u.IsBanned()
}

// CreateUser 后台代建 C 端账号——**邀请制下的唯一开户入口**（2026-09-15 新增）。
//
// 与公开注册共用 createUserWithPassword（同样的长度校验、唯一性检查与 bcrypt 哈希），
// 但**不受 `auth.allow_register` 开关限制**：关掉的是"自己注册"，不是"管理员开号"。
// 契约见 shared-docs/04-用户体系与鉴权边界设计.md §4.2。
func (s *UserAdminService) CreateUser(req dto.AdminCreateUserRequest) (*dto.AdminCreatedUser, error) {
	u, err := createUserWithPassword(s.users, req.Username, req.Password, req.DisplayName)
	if err != nil {
		return nil, err
	}
	logger.L().Infow("user created by admin", "user", u.ID, "username", u.Username)
	return &dto.AdminCreatedUser{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
	}, nil
}
