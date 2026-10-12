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
	// RelicWeights / RelicWeightRules 圣遗物评分权重（V3，2026-09-27 新增）：
	// 来源 miao-plugin（MIT），见 ai/08；与 StatWeights（推荐词条）语义不同，不要混用
	RelicWeights     []CharacterRelicWeight     `json:"relicWeights"`
	RelicWeightRules []CharacterRelicWeightRule `json:"relicWeightRules"`
	// MPWeights 小助手口径权重（7 维，主键 = (characterId, element)；element 仅旅行者有值）
	MPWeights []CharacterRelicWeightMP `json:"mpWeights"`
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
			rec := item.Rec
			// ⚠️ **`subStats[].weight` 归后台所有**（2026-09-27）⇒ 导入必须"只补缺、不覆盖" ✗：
			//    ⚠️ 更正（2026-10-12）：本注释原写"**评分权重就是 `subStats[].weight`**" —— **已过期** ✗。
			//    评分（`ScoreRelic`）走的是 `character_relic_weights`（10 维，另一个表 ✓），
			//    **不读这里的 weight** ✓（见 relic_score.go 的 `baseRelicWeight`，以及 model.go 里两张表的语义说明 ✓）。
			//    这里的 weight 只影响**推荐词条的排序/展示与 AI 上下文** ✓ —— 但它仍是后台的人工数据，
			//    所以"只补缺、不覆盖"的规则**不变** ✓。
			// 生成器产出的是**老的 10 分制优先级** ✗，一旦直接 UpdateAll，会把后台/迁移后的
			// 评分权重整片覆盖成 10 分制 ⇒ **分数静默全错** ✗✗。
			// 规则：① 同 statKey 的行，保留后台的 weight / tier ✓
			//      ② 后台有、导入件没有的行（含迁移补出来的维度）也要留住 ✓
			//      ③ 旅行者的 `byElement` 覆盖必须留住 ✓（否则七形态会被算法覆盖成一套 ✗）
			var existing CharacterStatWeights
			if err := tx.First(&existing, "character_id = ?", item.CharacterID).Error; err == nil && existing.Payload != "" {
				var old StatRecommendation
				if err := json.Unmarshal([]byte(existing.Payload), &old); err == nil {
					oldByKey := make(map[string]StatWeightItem, len(old.SubStats))
					for _, s := range old.SubStats {
						oldByKey[s.StatKey] = s
					}
					newByKey := make(map[string]bool, len(rec.SubStats))
					for i := range rec.SubStats {
						newByKey[rec.SubStats[i].StatKey] = true
						if o, ok := oldByKey[rec.SubStats[i].StatKey]; ok {
							rec.SubStats[i].Weight = o.Weight
							if o.Tier != "" {
								rec.SubStats[i].Tier = o.Tier
							}
						}
					}
					for _, s := range old.SubStats {
						if s.StatKey != "" && !newByKey[s.StatKey] {
							rec.SubStats = append(rec.SubStats, s)
						}
					}
					if len(old.ByElement) > 0 {
						rec.ByElement = old.ByElement
					}
				}
			}
			payload, err := json.Marshal(rec)
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
		// 评分权重：主键即 character_id，OnConflict UpdateAll 天然幂等
		if err := upsertBatch(tx, p.RelicWeights, upsertBatchSize); err != nil {
			return err
		}
		if err := upsertRelicWeightRules(tx, p.RelicWeightRules); err != nil {
			return err
		}
		// 小助手口径权重：复合主键 (character_id, element)，OnConflict UpdateAll 天然幂等
		if err := upsertBatch(tx, p.MPWeights, upsertBatchSize); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.Load() // 导入成功立即刷新内存
}

// upsertRelicWeightRules 覆盖规则幂等写入。
// 规则是自增主键，直接用 UpdateAll 会在重复导入时堆重复行 ⇒ 按业务唯一键冲突更新。
func upsertRelicWeightRules(tx *gorm.DB, rows []CharacterRelicWeightRule) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "character_id"}, {Name: "cond_type"}, {Name: "cond_value"}, {Name: "stat_key"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"character_name", "cond_op", "cond_num", "effect", "priority",
			"delta", "value", "value_min", "value_max", "dims", "require_stat", "require_min",
			"note", "source", "source_ref", "fetched_at", "updated_at",
		}),
	}).Create(&rows).Error
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
