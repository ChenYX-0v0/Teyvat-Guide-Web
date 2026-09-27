package asset

// 游戏资产映射表：ID/hash → 人类可读信息
// 数据存 MySQL（新版本角色只需补数据，不改代码），运行时全量加载进内存

import "time"

// Character 角色（avatarId → 展示信息）
type Character struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	EnName    string    `gorm:"size:64;not null" json:"enName"`
	Element   string    `gorm:"size:16;not null" json:"element"`
	Rarity    int       `gorm:"not null" json:"rarity"`
	Icon      string    `gorm:"size:128;not null" json:"icon"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Weapon 武器
type Weapon struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	EnName    string    `gorm:"size:64;not null" json:"enName"`
	Rarity    int       `gorm:"not null" json:"rarity"`
	Type      string    `gorm:"size:32;not null" json:"type"`
	Icon      string    `gorm:"size:128;not null" json:"icon"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RelicSet 圣遗物套装
type RelicSet struct {
	SetID     int       `gorm:"primaryKey" json:"setId"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Icon      string    `gorm:"size:128;not null" json:"icon"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RelicSetEffect 圣遗物套装 2 / 4 件效果（V2.2，2026-09-15 新增；表名 relic_set_effects）
// 主键 (set_id, piece)。数据源 EnkaGameAssets 只有套装名与 2 件套数值、**没有效果描述文本**，
// 因此这一半只能人工整理录入（见 ai/05 §V2.2-e）。
type RelicSetEffect struct {
	SetID  int    `gorm:"primaryKey" json:"setId"`
	Piece  int    `gorm:"primaryKey" json:"piece"` // 2 | 4
	Effect string `gorm:"type:text;not null" json:"effect"`
	// Source 来源与核对状态（如「官方资料整理·待核对」）：上线前逐条核对，核对过改为正式来源
	Source    string    `gorm:"size:64;not null" json:"source"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TextMapEntry 文本映射（StableHash → 中文）
type TextMapEntry struct {
	Hash int64  `gorm:"primaryKey" json:"hash"`
	Text string `gorm:"size:255;not null" json:"text"`
}

// PropMap 属性 ID → 词条名
type PropMap struct {
	PropID  string `gorm:"primaryKey;size:32" json:"propId"`
	Name    string `gorm:"size:32;not null" json:"name"`
	Percent bool   `gorm:"not null" json:"percent"`
}

// FightProp 数字枚举 → PropID 映射（Enka fightPropMap 的数字键 → FIGHT_PROP_XXX）
type FightProp struct {
	NumID  string `gorm:"primaryKey;size:16" json:"numId"` // 如 "2000"
	PropID string `gorm:"size:48;not null" json:"propId"`  // 如 FIGHT_PROP_HP
}

// RelicSlot 圣遗物部位映射（EQUIP_BRACER → 生之花）
type RelicSlot struct {
	Pos  string `gorm:"primaryKey;size:32" json:"pos"` // EQUIP_BRACER 等
	Name string `gorm:"size:16;not null" json:"name"`  // 中文部位名
}

// Affix 圣遗物词缀表（ReliquaryAffixExcelConfigData）
// 用于解码 Enka appendPropIdList：数字 id → 词条类型（该 id 同时编码了强化档位）
type Affix struct {
	ID        int64   `gorm:"primaryKey" json:"id"`
	PropType  string  `gorm:"size:48;not null" json:"propType"` // FIGHT_PROP_XXX
	PropValue float64 `json:"propValue"`                        // 该档位数值
}

// StatWeightItem 推荐词条项（weight 越大越优先）
// Tier 属性分级：primary=主要属性 secondary=次要属性 空=未分级
type StatWeightItem struct {
	StatKey string `json:"statKey"` // FIGHT_PROP_XXX
	Weight  int    `json:"weight"`
	Tier    string `json:"tier,omitempty"` // primary | secondary | ""
}

// StatRecommendation 角色推荐强化词条（副词条权重 + 主词条推荐）
type StatRecommendation struct {
	SubStats  []StatWeightItem  `json:"subStats"`
	MainStats map[string]string `json:"mainStats,omitempty"` // sand/goblet/circlet → 推荐主属性
}

// CharacterStatWeights 角色推荐词条（按角色存储，JSON payload）
type CharacterStatWeights struct {
	CharacterID int64     `gorm:"primaryKey;autoIncrement:false" json:"characterId"`
	Payload     string    `gorm:"type:json" json:"payload"` // StatRecommendation 序列化
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CharacterRelicWeight 圣遗物评分权重（V3，2026-09-27 新增；表名 character_relic_weights）
//
// ⚠️ 与 CharacterStatWeights（角色推荐词条）**语义不同、不可混用**：
//   - CharacterStatWeights：10 分制、5 类词条，用途是"给用户/提示词的推荐"；
//   - 本表：0~100 十维（含 生命/防御/精通/增伤/物伤/治疗），用途是"给圣遗物打分"。
//
// 数据来源 miao-plugin（MIT License, Copyright (c) 2023 Yoimiya），
// 详见 teyvat-docs/ai/08-圣遗物评分设计与落地.md；种子由 cmd/assetimport/gen-relic-weights.ps1 生成。
type CharacterRelicWeight struct {
	CharacterID int64  `gorm:"primaryKey;autoIncrement:false" json:"characterId"`
	Name        string `gorm:"size:32" json:"name,omitempty"`
	HP          int    `gorm:"not null;default:0" json:"hp"`
	ATK         int    `gorm:"not null;default:0" json:"atk"`
	DEF         int    `gorm:"not null;default:0" json:"def"`
	CPct        int    `gorm:"column:cpct;not null;default:0" json:"cpct"`
	CDmg        int    `gorm:"column:cdmg;not null;default:0" json:"cdmg"`
	Mastery     int    `gorm:"not null;default:0" json:"mastery"`
	Dmg         int    `gorm:"not null;default:0" json:"dmg"`
	Phy         int    `gorm:"not null;default:0" json:"phy"`
	Recharge    int    `gorm:"not null;default:0" json:"recharge"`
	Heal        int    `gorm:"not null;default:0" json:"heal"`
	// 来源与版本锚点：上游持续变动，必须能复现（AI/08 §9 风险 1）
	Source    string    `gorm:"size:64" json:"source,omitempty"`
	SourceRef string    `gorm:"size:96" json:"sourceRef,omitempty"`
	FetchedAt string    `gorm:"size:16" json:"fetchedAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Dim 按维度 key 取权重（hp/atk/def/cpct/cdmg/mastery/dmg/phy/recharge/heal）
func (w CharacterRelicWeight) Dim(key string) int {
	switch key {
	case "hp":
		return w.HP
	case "atk":
		return w.ATK
	case "def":
		return w.DEF
	case "cpct":
		return w.CPct
	case "cdmg":
		return w.CDmg
	case "mastery":
		return w.Mastery
	case "dmg":
		return w.Dmg
	case "phy":
		return w.Phy
	case "recharge":
		return w.Recharge
	case "heal":
		return w.Heal
	}
	return 0
}

// CharacterRelicWeightRule 评分权重的角色级覆盖规则（表名 character_relic_weight_rules）
//
// 把 miao-plugin 的 character/<角色>/artis.js 逻辑数据化，运营可在后台维护：
// 例：菲林斯 + 武器「血染荒城」→ 充能权重 -5；哥伦比娅 + 命座 ≥4 → 充能权重 -15（可叠加）。
type CharacterRelicWeightRule struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CharacterID   int64     `gorm:"uniqueIndex:idx_relic_rule;not null" json:"characterId"`
	CharacterName string    `gorm:"size:32" json:"characterName,omitempty"`
	CondType      string    `gorm:"uniqueIndex:idx_relic_rule;size:16;not null" json:"condType"` // weapon | cons
	CondOp        string    `gorm:"size:8;not null" json:"condOp"`                               // = | >=
	CondValue     string    `gorm:"uniqueIndex:idx_relic_rule;size:32;not null" json:"condValue"`
	StatKey       string    `gorm:"uniqueIndex:idx_relic_rule;size:16;not null" json:"statKey"` // 维度 key，如 recharge
	Delta         int       `gorm:"not null;default:0" json:"delta"`
	Note          string    `gorm:"size:32" json:"note,omitempty"` // 标签：专武 / 高命
	Source        string    `gorm:"size:64" json:"source,omitempty"`
	SourceRef     string    `gorm:"size:96" json:"sourceRef,omitempty"`
	FetchedAt     string    `gorm:"size:16" json:"fetchedAt,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Missing 翻译缺漏收集（补录依据，不阻塞查询）
type Missing struct {
	Kind      string    `gorm:"primaryKey;size:16" json:"kind"` // character|weapon|relic_set|text|prop|affix
	AssetID   string    `gorm:"primaryKey;size:64" json:"assetId"`
	FirstSeen time.Time `gorm:"not null" json:"firstSeen"`
	HitCount  int       `gorm:"not null" json:"hitCount"`
}
