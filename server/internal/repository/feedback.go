package repository

// 功能反馈持久化（2026-10-12）

import (
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
)

type FeedbackRepository struct{}

func NewFeedbackRepository() *FeedbackRepository { return &FeedbackRepository{} }

func (r *FeedbackRepository) db() *gorm.DB {
	return database.DB()
}

// Create 落库一条反馈（`ID` / `CreatedAt` 由 GORM 回填 ✓）。
// db 为 nil 时（未接数据库的降级模式）静默返回 nil ✓ —— 与其它仓储同一口径 ✓。
func (r *FeedbackRepository) Create(f *model.Feedback) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Create(f).Error
}
