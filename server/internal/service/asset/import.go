package asset

// 批量导入映射数据（JSON payload → MySQL upsert → 内存 reload）

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImportPayload 管理接口 / CLI 的导入载荷，按表分组，全部可选
type ImportPayload struct {
	Characters []Character    `json:"characters"`
	Weapons    []Weapon       `json:"weapons"`
	RelicSets  []RelicSet     `json:"relicSets"`
	TextMap    []TextMapEntry `json:"textMap"`
	PropMap    []PropMap      `json:"propMap"`
	Affixes    []Affix        `json:"affixes"`
}

const upsertBatchSize = 500

func (s *Service) Import(p ImportPayload) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertBatch(tx, p.Characters, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.Weapons, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.RelicSets, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.TextMap, 1000); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.PropMap, upsertBatchSize); err != nil {
			return err
		}
		return upsertBatch(tx, p.Affixes, upsertBatchSize)
	})
	if err != nil {
		return err
	}
	return s.Load() // 导入成功立即刷新内存
}

func upsertBatch[T any](tx *gorm.DB, rows []T, size int) error {
	if len(rows) == 0 {
		return nil
	}
	for start := 0; start < len(rows); start += size {
		end := min(start+size, len(rows))
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(rows[start:end]).Error; err != nil {
			return err
		}
	}
	return nil
}
