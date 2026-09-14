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

// Create 新建。
// ⚠️ Enabled 带 `default:true`：GORM 插入时会**跳过 false 零值**（该列取库默认值 true），
// 于是"新建时就取消上架"会落成启用（后台弹窗允许这么点）。插入后显式改正一次，
// 保证"新建即下架"能真的落成 enabled=0。
func (r *PresetQARepository) Create(item *model.PresetQA) error {
	if r.db() == nil {
		return nil
	}
	// 注意顺序：GORM 插入带 default 的字段后会把库里的默认值**回写进结构体**
	// （item.Enabled 会变成 true），所以"是否要未启用"必须在插入前记下来。
	wantDisabled := !item.Enabled
	if err := r.db().Create(item).Error; err != nil {
		return err
	}
	if wantDisabled {
		if err := r.db().Model(&model.PresetQA{}).
			Where("id = ?", item.ID).
			Update("enabled", false).Error; err != nil {
			return err
		}
		item.Enabled = false
	}
	return nil
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
