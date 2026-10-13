package model

import "time"

// CharacterMechanic 角色机制卡（阶段②，设计见 teyvat-docs/ai/07 TODO-20）
//
// 与「攻略卡」（CharacterGuide）的分工 —— 这是整件事的关键 ✓：
//   - 攻略卡 = 攻略页的**结论**（配队思路 / 毕业线 / 天赋顺序）
//   - 机制卡 = 从攻略页抽出的**事实**（提供什么 / 做不到什么 / 需要什么 / 边界 / 冲突）
//
// ⇒ 机制卡让模型**自己推**配队结论 ✓，不再死记某一页的固定说法 ✓（阶段③ 会把
// 攻略卡的「配队思路」结论降级/删除 ✓）。
//
// 三条硬约束（与攻略卡同一口径 ✓，改前先看 TODO-18/20 ✗）：
//  1. **刻意不进 asset.Service 的内存缓存** ✗ —— 每问直查库（character_id 唯一索引 ≈ 亚毫秒 ✓）
//     ⇒ 后台改完立即生效、不用 reload ✓。
//  2. **不存结论** ✗ —— 「班尼特加攻击力」是事实（存 ✓）；「班尼特适合魈队」是结论
//     （不存 ✗，留给模型推 ✓）。
//  3. **evidence / sources / _review 只进表与后台界面** ✗ —— 一律不拼进发给模型的文本
//     （提示词 v12 第 40~42 条要求这些块是"数据"；带来源会诱导模型说出"资料显示"✗）。
type CharacterMechanic struct {
	ID          uint64 `gorm:"primaryKey" json:"id"`
	CharacterID int64  `gorm:"uniqueIndex;not null" json:"characterId"`
	// Name 冗余存一份：后台列表不用 join `characters` 就能看出是哪位 ✓
	Name string `gorm:"size:64;not null" json:"name"`
	// Facts 机制卡原文 JSON（provides / cannot / limits / needs / scaling / conflicts /
	// shield / interruptResist / application / resonance ✓）—— 存原文、渲染时按字段取 ✓：
	// 以后加字段不用改表结构 ✓，而渲染器会**跳过 evidence** ✗。
	Facts string `gorm:"type:json" json:"facts"`
	// Review 复核状态：reviewed（人工过过 ✓）/ draft（初稿 ✓）。只进后台，不进注入文本 ✓
	Review      string    `gorm:"size:32" json:"review"`
	ContentHash string    `gorm:"size:64" json:"contentHash"` // 幂等导入用：变了才重写 ✓
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// MechanicRule 全局机制规则（抗性三段式 / 减抗叠加口径 / 同名四件套 / 共鸣表 ✓）。
//
// 为什么单独一张表而不是塞进角色卡 ✗：这些规则**所有角色共用** ✓，
// 而注入是**按关键词触发**的（问「减抗」才带上抗性规则 ✓）⇒ 需要一个「触发词」字段 ✓。
type MechanicRule struct {
	ID      uint64 `gorm:"primaryKey" json:"id"`
	RuleKey string `gorm:"uniqueIndex;size:64;not null" json:"ruleKey"`
	Title   string `gorm:"size:64" json:"title"`
	Content string `gorm:"type:text" json:"content"`
	// Triggers 触发词，逗号分隔（命中任一才注入 ✓）。空 = 永不注入（避免把整张表塞进上下文 ✗）
	Triggers    string    `gorm:"size:512" json:"triggers"`
	Confirmed   bool      `json:"confirmed"` // 人工确认过才算数 ✓（未确认的只入库、不注入 ✗）
	ContentHash string    `gorm:"size:64" json:"contentHash"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
