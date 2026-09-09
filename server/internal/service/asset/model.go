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

// Affix 圣遗物词缀表（ReliquaryAffixExcelConfigData）
// 用于解码 Enka appendPropIdList：数字 id → 词条类型（该 id 同时编码了强化档位）
type Affix struct {
	ID        int64   `gorm:"primaryKey" json:"id"`
	PropType  string  `gorm:"size:48;not null" json:"propType"` // FIGHT_PROP_XXX
	PropValue float64 `json:"propValue"`                        // 该档位数值
}

// Missing 翻译缺漏收集（补录依据，不阻塞查询）
type Missing struct {
	Kind      string    `gorm:"primaryKey;size:16" json:"kind"` // character|weapon|relic_set|text|prop|affix
	AssetID   string    `gorm:"primaryKey;size:64" json:"assetId"`
	FirstSeen time.Time `gorm:"not null" json:"firstSeen"`
	HitCount  int       `gorm:"not null" json:"hitCount"`
}
