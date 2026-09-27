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

// StatRecommendation 角色推荐强化词条（副词条权重 + 主词条推荐 + **评分权重**）
type StatRecommendation struct {
	SubStats  []StatWeightItem  `json:"subStats"`
	MainStats map[string]string `json:"mainStats,omitempty"` // sand/goblet/circlet → 推荐主属性
	// Score 圣遗物评分权重（2026-09-27 合并：评分权重**写在推荐属性里**，一份数据一个地方 ✓）
	Score *RelicScorePayload `json:"score,omitempty"`
}

// RelicScorePayload 评分权重块（小助手口径 7 维，key 与推荐词条一致 = `FIGHT_PROP_*`）
//
// 为什么放在推荐属性 payload 里：原先评分权重单独一张表（`character_relic_weights_mp`），
// 而"推荐词条优先级"与"评分权重"是同一批词条的两个数字 ⇒ 两处维护必然漂移 ✗（用户决定合并 ✓）。
//
// 元素维度**放进 JSON**：推荐属性表主键是一行一角色 ✗，而旅行者 7 个形态要分开算 ✓
// ⇒ 用 `ByElement` 表达（key = 风/岩/雷/草/水/火/冰；`Dims` 是通用/兜底行）。
type RelicScorePayload struct {
	// Dims 通用权重：`FIGHT_PROP_*` → 0~100（0 = 该角色不吃这一维）
	Dims map[string]int `json:"dims,omitempty"`
	// ByElement 仅旅行者用：元素 → 该形态的权重（覆盖 Dims）
	ByElement map[string]map[string]int `json:"byElement,omitempty"`
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

// 评分权重的效果类型（列 effect；历史行为空 ⇒ 按 delta 处理）
const (
	// RelicEffectDelta 命中即把某维加减 Delta（角色级规则，例：菲林斯 + 专武 → 充能 -5）
	RelicEffectDelta = "delta"
	// RelicEffectDeltaRefine 按武器精炼线性加某维：R1 ~ R5 从 ValueMin 增到 ValueMax，封顶 100
	// （上游 ArtisMarkCfg.weaponCfg 的"武器适配"）
	RelicEffectDeltaRefine = "delta_refine"
	// RelicEffectRaiseTo 把某维**只升不降**地抬到 Value（例：西风系列 → 暴击率 100）
	RelicEffectRaiseTo = "raise_to"
	// RelicEffectRaiseToMaxOf 把某维抬到 Dims 里最高者（例：绝缘 4 件 → 充能拉齐最高维）
	RelicEffectRaiseToMaxOf = "raise_to_max_of"
)

// CharacterRelicWeightRule 评分权重的覆盖规则（表名 character_relic_weight_rules）
//
// 把 miao-plugin 的规则数据化（**含三类动态规则**，2026-09-27 由硬编码改为落库）：
//   - 角色级（character/<角色>/artis.js）：菲林斯 + 武器「血染荒城」→ 充能 -5；哥伦比娅 + 命座 ≥4 → 充能 -15（可叠加）
//   - 通用级（**CharacterID = 0**，对所有角色生效）：
//     武器适配（ArtisMarkCfg.weaponCfg，按精炼线性）、西风系列 → 暴击 100、绝缘 4 件 → 充能拉齐最高维
//
// 求值顺序由 Priority 决定（小在前，同序按 ID）：先加减（10/20），后抬升（30/40）——
// 顺序会影响结果（如"先加攻击、再让充能拉齐最高维"），所以**不能只靠 ID 顺序**。
type CharacterRelicWeightRule struct {
	ID            int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	CharacterID   int64  `gorm:"uniqueIndex:idx_relic_rule;not null" json:"characterId"` // 0 = 通用于所有角色
	CharacterName string `gorm:"size:32" json:"characterName,omitempty"`
	// 条件类型：weapon（武器名精确匹配）| weapon_regex（武器名正则）| cons（命座数）| set（套装名 + 件数）
	CondType  string `gorm:"uniqueIndex:idx_relic_rule;size:16;not null" json:"condType"`
	CondOp    string `gorm:"size:8;not null" json:"condOp"` // = | >= | <=
	CondValue string `gorm:"uniqueIndex:idx_relic_rule;size:32;not null" json:"condValue"`
	// CondNum 数值条件：set 的件数阈值（如 ≥4）、cons 的命座数（也可用 CondValue 写数字，历史数据如此）
	CondNum int `json:"condNum,omitempty"`

	StatKey string `gorm:"uniqueIndex:idx_relic_rule;size:16;not null" json:"statKey"` // 目标维度 key，如 recharge

	Effect   string `gorm:"size:16;not null;default:delta" json:"effect,omitempty"`
	Priority int    `gorm:"not null;default:0" json:"priority,omitempty"`
	Delta    int    `json:"delta,omitempty"`               // delta
	Value    int    `json:"value,omitempty"`               // raise_to 的目标值
	ValueMin int    `json:"valueMin,omitempty"`            // delta_refine 的 R1 增量
	ValueMax int    `json:"valueMax,omitempty"`            // delta_refine 的 R5 增量
	Dims     string `gorm:"size:64" json:"dims,omitempty"` // raise_to_max_of：参照维度（逗号分隔）

	// 前置条件：仅当该维权重 ≥ RequireMin 时才生效（如武器适配只对攻击型角色：atk ≥ 1）
	RequireStat string `gorm:"size:16" json:"requireStat,omitempty"`
	RequireMin  int    `json:"requireMin,omitempty"`

	Note      string    `gorm:"size:32" json:"note,omitempty"` // 命中后给前端的标签：专武 / 高命 / 西风 / 绝缘4
	Source    string    `gorm:"size:64" json:"source,omitempty"`
	SourceRef string    `gorm:"size:96" json:"sourceRef,omitempty"`
	FetchedAt string    `gorm:"size:16" json:"fetchedAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// EffectOrDefault 空值按 delta 处理（兼容加列之前写入的历史行）
func (r CharacterRelicWeightRule) EffectOrDefault() string {
	if r.Effect == "" {
		return RelicEffectDelta
	}
	return r.Effect
}

// Missing 翻译缺漏收集（补录依据，不阻塞查询）
type Missing struct {
	Kind      string    `gorm:"primaryKey;size:16" json:"kind"` // character|weapon|relic_set|text|prop|affix
	AssetID   string    `gorm:"primaryKey;size:64" json:"assetId"`
	FirstSeen time.Time `gorm:"not null" json:"firstSeen"`
	HitCount  int       `gorm:"not null" json:"hitCount"`
}
