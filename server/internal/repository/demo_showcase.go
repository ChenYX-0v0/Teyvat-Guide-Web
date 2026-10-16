package repository

import (
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
)

// DemoShowcaseRepository 游客演示数据（demo_profiles / demo_characters）持久化。
// 只读 + 种子写入；演示数据不做物理删除，下架用 enabled=0 ✓
type DemoShowcaseRepository struct{}

func NewDemoShowcaseRepository() *DemoShowcaseRepository {
	return &DemoShowcaseRepository{}
}

func (r *DemoShowcaseRepository) db() *gorm.DB {
	return database.DB()
}

// Profile 取演示账号头（单行表，取 id 最小的一条；无数据返回 nil, nil）
func (r *DemoShowcaseRepository) Profile() (*model.DemoProfile, error) {
	if r.db() == nil {
		return nil, nil
	}
	var row model.DemoProfile
	err := r.db().Where("enabled = ?", true).Order("id asc").First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// ListEnabledCharacters 取启用的演示角色，按「sort + id」稳定排序 ✓
func (r *DemoShowcaseRepository) ListEnabledCharacters() ([]model.DemoCharacter, error) {
	if r.db() == nil {
		return nil, nil
	}
	var list []model.DemoCharacter
	err := r.db().
		Where("enabled = ?", true).
		Order("sort asc, id asc").
		Find(&list).Error
	return list, err
}

// CountProfiles 现有条数（种子判断用）
func (r *DemoShowcaseRepository) CountProfiles() (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	var n int64
	err := r.db().Model(&model.DemoProfile{}).Count(&n).Error
	return n, err
}

// CountCharacters 现有条数（种子判断用）
func (r *DemoShowcaseRepository) CountCharacters() (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	var n int64
	err := r.db().Model(&model.DemoCharacter{}).Count(&n).Error
	return n, err
}

// SeedProfiles 批量写入演示账号头（只在表为空时调用 ⇒ 天然幂等 ✓）
func (r *DemoShowcaseRepository) SeedProfiles(items []model.DemoProfile) error {
	if r.db() == nil || len(items) == 0 {
		return nil
	}
	return r.db().Create(&items).Error
}

// SeedCharacters 批量写入演示角色（只在表为空时调用 ⇒ 天然幂等 ✓）
func (r *DemoShowcaseRepository) SeedCharacters(items []model.DemoCharacter) error {
	if r.db() == nil || len(items) == 0 {
		return nil
	}
	return r.db().Create(&items).Error
}
