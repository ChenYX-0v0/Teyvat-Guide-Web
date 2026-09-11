package service

// 用户管理（供后台运维系统经内网接口调用）
// 契约：04-用户体系与鉴权边界设计 §4

import (
	"errors"
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

// List 分页查询用户（keyword 匹配 用户名 / 昵称 / 游戏 UID），附带 token 消耗聚合
func (s *UserAdminService) List(keyword string, page, pageSize int) (*dto.AdminUserList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	users, total, err := s.users.ListUsers(keyword, (page-1)*pageSize, pageSize)
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

	items := make([]dto.AdminUserItem, 0, len(users))
	for _, u := range users {
		item := dto.AdminUserItem{
			ID:          u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Status:      u.Status,
			CreatedAt:   u.CreatedAt.Format(time.RFC3339),
			TotalTokens: tokenMap[u.ID],
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
