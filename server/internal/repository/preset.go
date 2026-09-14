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

// ---------- 后台维护（读写） ----------

// ListAll 全部条目（**含已下架**），排序与 C 端一致：分组 → 组内排序 → id
func (r *PresetQARepository) ListAll() ([]model.PresetQA, error) {
	if r.db() == nil {
		return nil, nil
	}
	var list []model.PresetQA
	err := r.db().
		Order("scene_key asc, sort asc, id asc").
		Find(&list).Error
	return list, err
}

// FindByID 按 ID 取（不存在返回 nil, nil）
func (r *PresetQARepository) FindByID(id uint) (*model.PresetQA, error) {
	if r.db() == nil {
		return nil, nil
	}
	var item model.PresetQA
	err := r.db().First(&item, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// Create 新建
func (r *PresetQARepository) Create(item *model.PresetQA) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Create(item).Error
}

// Update 按字段更新。
// 注意：**不依赖 RowsAffected** 判断是否存在——MySQL 在"值没变化"时返回 0 行，
// 会被误判成记录不存在；存在性由调用方先 FindByID 判断。
func (r *PresetQARepository) Update(id uint, fields map[string]any) error {
	if r.db() == nil || len(fields) == 0 {
		return nil
	}
	return r.db().Model(&model.PresetQA{}).Where("id = ?", id).Updates(fields).Error
}
