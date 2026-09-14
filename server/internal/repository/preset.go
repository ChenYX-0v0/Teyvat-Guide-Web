package repository

import (
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
)

// PresetQARepository 预设问答持久化
type PresetQARepository struct{}

func NewPresetQARepository() *PresetQARepository {
	return &PresetQARepository{}
}

func (r *PresetQARepository) db() *gorm.DB {
	return database.DB()
}

// ListEnabled 取全部启用的预设问答，按「分组 + 组内排序 + id」稳定排序
func (r *PresetQARepository) ListEnabled() ([]model.PresetQA, error) {
	if r.db() == nil {
		return nil, nil
	}
	var list []model.PresetQA
	err := r.db().
		Where("enabled = ?", true).
		Order("scene_key asc, sort asc, id asc").
		Find(&list).Error
	return list, err
}

// Count 现有条数（种子判断用）
func (r *PresetQARepository) Count() (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	var n int64
	if err := r.db().Model(&model.PresetQA{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// Seed 批量插入种子数据
func (r *PresetQARepository) Seed(items []model.PresetQA) error {
	if r.db() == nil || len(items) == 0 {
		return nil
	}
	return r.db().Create(&items).Error
}
