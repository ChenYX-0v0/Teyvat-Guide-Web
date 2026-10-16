package model

import "time"

// DemoCharacter 游客演示模式的「模拟角色档案」。
//
// 为什么单独建表（**不要**塞进 player_characters）✗：
//   - `player_characters` 的语义是"真实用户累积池"，**只增不改、永不物理删除**，
//     塞演示数据会污染真库，而且分不清哪条是真人哪条是演示；
//   - 游客未登录（uid=0），`/api/genshin/player`、`/api/genshin/characters` 对 uid=0 直接 400，
//     复用真实档案链路必然报错。
//
// 这条链路**不调用 AI 服务、不查 Enka、不落库会话** ✓：
// 角色卡与「AI 分析」都是运营在库里预置好的内容（改文案改库即可，不用发版）✓。
type DemoCharacter struct {
	ID      uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	CharKey string `gorm:"size:32;not null;uniqueIndex" json:"char_key"` // 稳定业务键（如 mavuika），前端 key / 幂等种子用
	Sort    int    `gorm:"not null;default:0" json:"sort"`               // 展示顺序

	Name    string `gorm:"size:64;not null" json:"name"`
	Element string `gorm:"size:16;not null" json:"element"` // 中文元素名（火 / 水 / 风 / 雷 / 冰 / 岩 / 草）
	Rarity  int    `gorm:"not null;default:5" json:"rarity"`
	Role    string `gorm:"size:32;not null" json:"role"` // 展示用定位标签（站场主C / 盾辅 …）

	// CardJSON dto.CharacterCard 的序列化 ✓ —— **与真实档案同构**，
	// 前端因此能直接把它当展柜角色卡渲染（面板 / 武器 / 圣遗物 / 评分全部复用现有组件）✓
	CardJSON string `gorm:"type:json;not null" json:"card_json"`
	// Analysis 预置的「AI 分析」正文（纯文本，前端按纯文本逐字回放，**不调模型**）✓
	Analysis string `gorm:"type:text;not null" json:"analysis"`
	Enabled  bool   `gorm:"not null;default:true" json:"enabled"` // 下架开关；不做物理删除

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DemoCharacter) TableName() string { return "demo_characters" }

// DemoProfile 游客演示的「模拟旅行者账号」头信息（**单行表**，id=1）✓。
//
// 与 demo_characters 一样只在演示页使用；真实用户档案走 player_profile_snapshots ✓。
type DemoProfile struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Nickname     string `gorm:"size:64;not null" json:"nickname"`
	Signature    string `gorm:"size:128;not null" json:"signature"`
	Level        int    `gorm:"not null;default:0" json:"level"` // 冒险等阶
	WorldLevel   int    `gorm:"not null;default:0" json:"world_level"`
	Achievements int    `gorm:"not null;default:0" json:"achievements"`
	// AbyssJSON dto.AbyssSummary 的序列化；留空 = 演示页不渲染这一块 ✓（"没有" ≠ "0 层"）
	AbyssJSON string `gorm:"type:json" json:"abyss_json"`
	// Notice 演示页顶部的提示语（如"演示内容 · 非实时账号数据"）。
	// 放库不写死：措辞随产品/合规要求会改，改库即可、不用发版 ✓
	Notice  string `gorm:"size:255;not null;default:''" json:"notice"`
	Enabled bool   `gorm:"not null;default:true" json:"enabled"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DemoProfile) TableName() string { return "demo_profiles" }
