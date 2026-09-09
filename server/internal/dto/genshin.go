package dto

import "teyvat-guide/server/internal/service/asset"

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
}

// CharacterCard 展柜角色卡
type CharacterCard struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	Element       string      `json:"element"`
	Rarity        int         `json:"rarity"`
	Level         int         `json:"level"`
	Constellation int         `json:"constellation"`
	Fetter        int         `json:"fetter"`
	IconURL       string      `json:"iconUrl,omitempty"`
	Stats         []StatInfo  `json:"stats,omitempty"` // 面板属性
	Weapon        *WeaponInfo `json:"weapon,omitempty"`
	Relics        []RelicInfo `json:"relics,omitempty"`
}

// StatInfo 面板属性行（名称 + 格式化后的值）
type StatInfo struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type WeaponInfo struct {
	Name   string `json:"name"`
	Rarity int    `json:"rarity"`
	Level  int    `json:"level"`
	Affix  int    `json:"affix"` // 精炼等级
}

type RelicInfo struct {
	Name     string         `json:"name"`
	Set      string         `json:"set,omitempty"` // 套装名
	Rarity   int            `json:"rarity"`
	Level    int            `json:"level"`
	Pos      string         `json:"pos"` // 部位（EQUIP_BRACER 等，前端映射文案）
	MainStat string         `json:"mainStat,omitempty"`
	SubStats []RelicSubStat `json:"subStats,omitempty"`
}

// RelicSubStat 副词条（含强化次数）
type RelicSubStat struct {
	Name  string `json:"name"`  // 翻译后词条名
	Value string `json:"value"` // 当前值（含百分比符号）
	Count int    `json:"count"` // 强化次数（含初始获得）
}

// 资产管理出入参（复用 asset 包模型）
type (
	AssetImportPayload = asset.ImportPayload
	AssetMissingEntry  = asset.Missing
)
