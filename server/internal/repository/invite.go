package repository

// 邀请码持久化（2026-09-28）

import (
	"errors"
	"time"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
)

// ErrInviteTaken 邀请码已被（并发地）用掉 —— 由条件更新影响行数为 0 判定 ✓
var ErrInviteTaken = errors.New("invite code already taken")

type InviteRepository struct{}

func NewInviteRepository() *InviteRepository { return &InviteRepository{} }

func (r *InviteRepository) db() *gorm.DB { return database.DB() }

// Create 新建邀请码（主键冲突即返回错误，由上层转成"重复"✓）
func (r *InviteRepository) Create(code *model.InviteCode) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Create(code).Error
}

// Get 按码查询（未命中返回 (nil, nil)，与用户仓储同一约定 ✓）
func (r *InviteRepository) Get(code string) (*model.InviteCode, error) {
	if r.db() == nil {
		return nil, nil
	}
	var c model.InviteCode
	err := r.db().First(&c, "code = ?", code).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// List 列表（后台用）：unused=true 只列"可用（启用且未使用）"的 ✓
func (r *InviteRepository) List(unused bool, limit int) ([]model.InviteCode, error) {
	if r.db() == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := r.db().Model(&model.InviteCode{}).Order("created_at DESC").Limit(limit)
	if unused {
		q = q.Where("enabled = ? AND used_at IS NULL", true)
	}
	var out []model.InviteCode
	err := q.Find(&out).Error
	return out, err
}

// CountUsable 可用码数量（后台概览用）
func (r *InviteRepository) CountUsable() (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	var n int64
	err := r.db().Model(&model.InviteCode{}).Where("enabled = ? AND used_at IS NULL", true).Count(&n).Error
	return n, err
}

// SetEnabled 停用 / 启用（返回影响行数；0 = 码不存在）
func (r *InviteRepository) SetEnabled(code string, enabled bool) (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	res := r.db().Model(&model.InviteCode{}).
		Where("code = ? AND enabled <> ?", code, enabled).
		Updates(map[string]any{"enabled": enabled, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

// CreateUserWithInvite **原子完成"占用邀请码 + 建用户"** ✓✓
//
// 顺序刻意是"先建用户、再占用码"，且都在**同一个事务**里：
//  1. 建用户失败（比如手机号撞唯一索引）⇒ 事务回滚 ⇒ **码没被烧掉** ✓；
//  2. 占用用条件更新 `used_at IS NULL` ⇒ 影响行数 0 说明码已被别人用掉 ⇒ 返回 ErrInviteTaken
//     ⇒ 事务回滚 ⇒ **用户也不会被建出来** ✓（并发下不会两个账号共用一个码 ✓）。
func (r *InviteRepository) CreateUserWithInvite(u *model.User, code, phone string) error {
	db := r.db()
	if db == nil {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		now := time.Now()
		res := tx.Model(&model.InviteCode{}).
			Where("code = ? AND enabled = ? AND used_at IS NULL", code, true).
			Updates(map[string]any{
				"used_at":    now,
				"used_by":    u.ID,
				"used_phone": phone,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInviteTaken
		}
		return nil
	})
}
