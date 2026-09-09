package repository

// 用户持久化

import (
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

// EnsureAdmin 将指定用户名提升为管理员（幂等；种子账号迁移后补挂 role 用）
func (r *UserRepository) EnsureAdmin(username string) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Model(&model.User{}).
		Where("username = ? AND role <> ?", username, "admin").
		Update("role", "admin").Error
}
