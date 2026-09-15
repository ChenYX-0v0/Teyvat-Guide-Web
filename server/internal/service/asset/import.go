package asset

// 批量导入映射数据（JSON payload → MySQL upsert → 内存 reload）

import (
	"encoding/json"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StatWeightImport 推荐词条导入项
type StatWeightImport struct {
	CharacterID int64              `json:"characterId"`
	Rec         StatRecommendation `json:"recommendation"`
}

// ImportPayload 管理接口 / CLI 的导入载荷，按表分组，全部可选
type ImportPayload struct {
	Characters []Character `json:"characters"`
	Weapons    []Weapon    `json:"weapons"`
	RelicSets  []RelicSet  `json:"relicSets"`
	// RelicSetEffects 圣遗物套装 2/4 件效果（V2.2，2026-09-15 新增）：
	// 数据源（EnkaGameAssets）没有效果文本，只能人工整理录入，见 ai/05 V2.2-e
	RelicSetEffects []RelicSetEffect   `json:"relicSetEffects"`
	TextMap         []TextMapEntry     `json:"textMap"`
	PropMap         []PropMap          `json:"propMap"`
	FightProps      []FightProp        `json:"fightProps"`
	RelicSlots      []RelicSlot        `json:"relicSlots"`
	Affixes         []Affix            `json:"affixes"`
	StatWeights     []StatWeightImport `json:"statWeights"`
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
		if err := upsertBatch(tx, p.RelicSetEffects, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.TextMap, 1000); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.PropMap, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.FightProps, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.RelicSlots, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertBatch(tx, p.Affixes, upsertBatchSize); err != nil {
			return err
		}
		for _, item := range p.StatWeights {
			payload, err := json.Marshal(item.Rec)
			if err != nil {
				return err
			}
			row := CharacterStatWeights{
				CharacterID: item.CharacterID,
				Payload:     string(payload),
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
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
