package dto

import (
	"time"

	"teyvat-guide/server/internal/service/asset"
)

// PlayerProfile 玩家档案（Enka 数据源）
type PlayerProfile struct {
	Source       string          `json:"source"`
	UID          string          `json:"uid"`
	Nickname     string          `json:"nickname"`
	Level        int             `json:"level"`
	Signature    string          `json:"signature"`
	WorldLevel   int             `json:"worldLevel"`
	Achievements int             `json:"achievements"`
	AvatarURL    string          `json:"avatarUrl,omitempty"`
	Characters   []CharacterCard `json:"characters"`
	Warnings     []string        `json:"warnings,omitempty"`

	// 终局玩法概要（2026-09-27 新增）：Enka playerInfo 直读，**只有概要、没有明细**。
	// 玩家隐藏战绩时这里为 nil（"没有" ≠ "0 层/0 星"）。
	// ⚠️ 明细（每间队伍、怪物阵容）Enka 不提供，需另找数据源。
	Abyss   *AbyssSummary   `json:"abyss,omitempty"`
	Theater *TheaterSummary `json:"theater,omitempty"`
	Stygian *StygianSummary `json:"stygian,omitempty"`
}

// AbyssSummary 深境螺旋概要：打到第几层第几间 + 星数
// （Enka 字段：towerFloorIndex / towerLevelIndex / towerStarIndex）
type AbyssSummary struct {
	Floor int `json:"floor"` // 层（1~12）
	Room  int `json:"room"`  // 间（1~3）
	Stars int `json:"stars"` // 星数（满星 = 层 × 3）
}

// TheaterSummary 幻想真境剧诗概要（Enka 字段：theaterActIndex / theaterModeIndex / theaterStarIndex）
type TheaterSummary struct {
	Act   int `json:"act"`   // 幕 / 期序号
	Mode  int `json:"mode"`  // 模式 id（Enka 原值，未翻译）
	Stars int `json:"stars"` // 星数
}

// StygianSummary 幽境危战概要（Enka 字段：stygianIndex / stygianSeconds / stygianId）
type StygianSummary struct {
	Index   int `json:"index"`   // 难度 / 期序号（Enka 原值）
	Seconds int `json:"seconds"` // 用时（秒）
	ID      int `json:"id"`      // 关卡 id（Enka 原值）
}

// CharacterCollection 玩家「已收录角色」集合（2026-10-11）。
//
// 口径（用户明确要求）：**只增/只改，不删** ✓ —— 游戏里展柜最多 12 位 ✓，
// 用户通过反复更换展柜，可以把自己的角色**逐步全部导入**进来 ✓。
// 落库见 model.PlayerCharacter（与档案快照**分表** ✓，快照被清空不影响这里 ✓）。
type CharacterCollection struct {
	UID string `json:"uid"`
	// Total 已收录总数（只增不减 ✓）
	Total int `json:"total"`
	// ShowcaseCount 其中**当前仍在展柜**里的数量（≤12 ✓，其余是历史上见过但已移出的 ✓）
	ShowcaseCount int `json:"showcaseCount"`
	// Characters 展柜中的在前 ✓，同组内按最近见到倒序 ✓
	Characters []CollectedCharacter `json:"characters"`
}

// CollectedCharacter 已收录角色 = 角色卡 + 收录元信息 ✓
// （嵌入 CharacterCard ⇒ JSON 平铺 ✓，前端可以直接当角色卡用 ✓）
type CollectedCharacter struct {
	CharacterCard
	// InShowcase 当前是否在展柜里（false ⇒ 数据停在 LastSeenAt 那一刻 ✓，可能过时 ✗）
	InShowcase bool `json:"inShowcase"`
	// FirstSeenAt / LastSeenAt 首次收录 / 最近一次在展柜里见到的时间 ✓
	FirstSeenAt time.Time `json:"firstSeenAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

// CharacterCard 展柜角色卡
type CharacterCard struct {
	ID            int64               `json:"id"`
	Name          string              `json:"name"`
	Element       string              `json:"element"`
	Rarity        int                 `json:"rarity"`
	Level         int                 `json:"level"`
	Constellation int                 `json:"constellation"`
	Fetter        int                 `json:"fetter"`
	Talents       *TalentInfo         `json:"talents,omitempty"` // 天赋等级（普攻 / 战技 / 爆发）
	IconURL       string              `json:"iconUrl,omitempty"`
	Stats         []StatInfo          `json:"stats,omitempty"`       // 面板属性
	StatWeights   *StatRecommendation `json:"statWeights,omitempty"` // 推荐强化词条
	Weapon        *WeaponInfo         `json:"weapon,omitempty"`
	Relics        []RelicInfo         `json:"relics,omitempty"`
	// 圣遗物评分汇总（V3，2026-09-27；评分口径见 teyvat-docs/ai/08）
	RelicScoreTotal float64 `json:"relicScoreTotal,omitempty"` // 5 件总分（0~330）
	RelicScoreAvg   float64 `json:"relicScoreAvg,omitempty"`   // 平均分（0~66，分档按它）
	RelicScoreClass string  `json:"relicScoreClass,omitempty"` // 档位：D/C/B/A/S/SS/SSS/ACE/MAX
	RelicWordCount  float64 `json:"relicWordCount,omitempty"`  // 总词条数（均值口径）
	RelicCritScore  float64 `json:"relicCritScore,omitempty"`  // 双暴分（暴击×2+暴伤，仅副词条）
}

// StatWeightEntry 推荐强化词条（已翻译为中文词条名，weight 越大越优先）
// Tier 属性分级：primary=主要 secondary=次要 空=未分级
type StatWeightEntry struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
	Tier   string `json:"tier,omitempty"`
}

// StatRecommendation 角色推荐强化词条
type StatRecommendation struct {
	SubStats  []StatWeightEntry `json:"subStats,omitempty"`
	MainStats map[string]string `json:"mainStats,omitempty"` // sand/goblet/circlet → 推荐主属性
}

// TalentInfo 天赋等级（普通攻击 / 元素战技 / 元素爆发）
type TalentInfo struct {
	Normal int `json:"normal"`
	Skill  int `json:"skill"`
	Burst  int `json:"burst"`
}

// StatInfo 面板属性行（名称 + 格式化后的值）
// Tier 属性分级：primary/secondary/空（未分级），来自角色推荐词条的 tierByName
type StatInfo struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Tier  string `json:"tier,omitempty"`
}

type WeaponInfo struct {
	Name   string `json:"name"`
	Rarity int    `json:"rarity"`
	Level  int    `json:"level"`
	Affix  int    `json:"affix"` // 精炼等级
}

type RelicInfo struct {
	Name         string         `json:"name"`
	Set          string         `json:"set,omitempty"`  // 套装名
	Slot         string         `json:"slot,omitempty"` // 部位中文名（生之花 / 时之沙 …）
	Rarity       int            `json:"rarity"`
	Level        int            `json:"level"`
	Pos          string         `json:"pos"` // 部位（EQUIP_BRACER 等，前端映射文案）
	MainStat     string         `json:"mainStat,omitempty"`
	MainStatTier string         `json:"mainStatTier,omitempty"` // 主词条分级：primary/secondary/空
	SubStats     []RelicSubStat `json:"subStats,omitempty"`
	// 部位图标：Enka 图床地址（如 .../ui/UI_RelicIcon_15005_4.png），供紧凑卡片展示部位
	IconURL string `json:"iconUrl,omitempty"`
	// 圣遗物评分（V3，2026-09-27；无权重配置的角色这些字段缺省为 0/空，前端整块不渲染）
	Score     float64 `json:"score,omitempty"`     // 单件分（0~66）
	ScorePct  float64 `json:"scorePct,omitempty"`  // 占该部位理论上限的百分比（0~100）
	ScoreCls  string  `json:"scoreCls,omitempty"`  // 档位：D/C/B/A/S/SS/SSS/ACE/MAX
	WordCount float64 `json:"wordCount,omitempty"` // 总词条数（均值口径）
	CritScore float64 `json:"critScore,omitempty"` // 双暴分（暴击×2+暴伤）
	WeightTag string  `json:"weightTag,omitempty"` // 命中的权重规则标签（专武/高命/西风/绝缘4）
}

// RelicSubStat 副词条（含强化次数）
type RelicSubStat struct {
	Name  string `json:"name"`           // 翻译后词条名
	Value string `json:"value"`          // 当前值（含百分比符号）
	Count int    `json:"count"`          // 强化次数（含初始获得）
	Tier  string `json:"tier,omitempty"` // 分级：primary/secondary/空
}

// 资产管理出入参（复用 asset 包模型）
type (
	AssetImportPayload = asset.ImportPayload
	AssetMissingEntry  = asset.Missing
)
