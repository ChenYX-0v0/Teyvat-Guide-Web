package repository

// 用户持久化

import (
	"time"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
)

type UserRepository struct{}

func NewUserRepository() *UserRepository { return &UserRepository{} }

func (r *UserRepository) db() *gorm.DB {
	return database.DB()
}

// Count 用户总数（种子账号判断用）
func (r *UserRepository) Count() (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	var n int64
	err := r.db().Model(&model.User{}).Count(&n).Error
	return n, err
}

// Create 创建用户
func (r *UserRepository) Create(u *model.User) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Create(u).Error
}

// FindByUsername 按用户名查询
func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	if r.db() == nil {
		return nil, nil
	}
	var u model.User
	err := r.db().First(&u, "username = ?", username).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// FindByID 按 ID 查询
func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	if r.db() == nil {
		return nil, nil
	}
	var u model.User
	err := r.db().First(&u, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// TouchLogin 记录最近登录时间
func (r *UserRepository) TouchLogin(id uint) error {
	if r.db() == nil {
		return nil
	}
	now := time.Now()
	return r.db().Model(&model.User{}).Where("id = ?", id).
		Update("last_login_at", now).Error
}

// ListUsers 分页查询用户；keyword 匹配 用户名 / 昵称 / 游戏 UID。
//
// includeIDs 的语义必须区分「不过滤」与「筛出来是空集」：
//   - nil     → 不过滤（返回所有用户）
//   - 非 nil  → 只返回这些 ID（**空切片表示没有任何用户命中**，返回 0 条）
//
// 用量 / 无关问题这类条件是**按消息聚合**出来的，先由 ChatRepository 算出候选 ID 集合，
// 再在这里参与 title 之外的过滤，这样 total 与分页天然一致。
func (r *UserRepository) ListUsers(keyword string, includeIDs []uint, offset, limit int) ([]model.User, int64, error) {
	if r.db() == nil {
		return nil, 0, nil
	}
	if includeIDs != nil && len(includeIDs) == 0 {
		return nil, 0, nil
	}

	q := r.db().Model(&model.User{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR display_name LIKE ? OR genshin_uid LIKE ?", like, like, like)
	}
	if includeIDs != nil {
		q = q.Where("id IN ?", includeIDs)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.User
	err := q.Order("id asc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// UpdateStatus 设置用户状态（active / banned），幂等
func (r *UserRepository) UpdateStatus(id uint, status string) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Model(&model.User{}).
		Where("id = ? AND status <> ?", id, status).
		Update("status", status).Error
}

// CountByID 判断用户是否存在
func (r *UserRepository) CountByID(id uint) (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	var n int64
	err := r.db().Model(&model.User{}).Where("id = ?", id).Count(&n).Error
	return n, err
}

// UpdateBinding 更新用户绑定的游戏 UID（uid 传空即解绑）
func (r *UserRepository) UpdateBinding(userID uint, uid, server string) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{"genshin_uid": uid, "genshin_server": server}).Error
}
