package model

import "time"

// CharacterGuide 角色攻略卡（阶段②，设计见 teyvat-docs/ai/07 TODO-18）
//
// 三条硬约束（都是定过的，改前先看 TODO-18 ✗）：
//  1. **刻意不进 asset.Service 的内存缓存** ✗ —— 主服务每问**直查库**（character_id 唯一索引，
//     亚毫秒 ✓）⇒ 后台改完**立即生效**、不用 reload ✓，也顺带绕开「两个 asset.Service 实例」
//     那个老坑（见 deploy/README §3.9 的 reload 说明 ✓）。
//  2. **不存主/副词条** ✗ —— 那部分以 `asset.CharacterStatWeights` 为准（能分级、能参与评分 ✓），
//     攻略卡只给"为什么这么堆"的理由 ⇒ 两处都写必然打架。
//  3. **来源三件套只进表与后台界面** ✗ —— 不要拼进发给模型的文本（提示词 v11 第 9/11/39 条
//     禁止"社区口径 / 以官方为准"这类出戏说法 ✓）。
type CharacterGuide struct {
	ID uint64 `gorm:"primaryKey" json:"id"`
	// CharacterID ＋ FormElement 是**联合唯一键** ✓（uk_guide_char_form ✓）
	//
	// ⚠️ 为什么不是「character_id 单列唯一」✗（2026-10-14 改 ✓）：
	//   旅行者是**多形态角色** ✓ —— biligame 攻略页写作「旅行者/风/攻略」等 **8 篇**
	//   （风 / 岩 / 雷 / 草 / 水 / 火 / 冰 ＋ 无派蒙「原人」✓，见 `scripts/extract-character-guides.mjs` 的 `formElement` ✓）。
	//   原先按 character_id 唯一 ✗ ⇒ 8 篇互相**覆盖**，库里只剩最后写进去的那篇 ✓
	//   （线上实测：只剩「旅行者/冰」✓，草主那条人工毕业线 210% 直接丢了 ✗）。
	CharacterID int64 `gorm:"not null;uniqueIndex:uk_guide_char_form,priority:1" json:"characterId"`
	// FormElement 形态（元素）：旅行者 = 风/岩/雷/草/水/火/冰/原人 ✓；其他角色一律**空串** ✓
	//   ⚠️ 空串而不是 NULL ✗ —— MySQL 唯一索引里 NULL 不参与去重 ⇒ 会插出重复行 ✓
	FormElement string `gorm:"size:16;not null;default:'';uniqueIndex:uk_guide_char_form,priority:2" json:"formElement"`
	// Name 冗余存一份：后台列表不用 join `characters` 就能看是哪个角色 ✓
	Name string `gorm:"size:64;not null" json:"name"`

	Role       string `gorm:"type:text" json:"role"`       // 定位 + 体系（散文）
	Talents    string `gorm:"type:text" json:"talents"`    // 天赋加点顺序 + 理由
	WeaponPick string `gorm:"type:text" json:"weaponPick"` // 武器（毕业 + 可选）
	RelicPick  string `gorm:"type:text" json:"relicPick"`  // 圣遗物套装 + 理由（**不含主/副词条** ✗）
	TeamPick   string `gorm:"type:text" json:"teamPick"`   // 配队思路
	// TeamFrom 这条配队从哪来：page=本页原文 ✓ / reverse=从其他角色攻略页反推（该页自己没写）✓
	TeamFrom   string `gorm:"size:16" json:"teamFrom"`
	Graduation string `gorm:"type:text" json:"graduation"` // 毕业线（数值=社区口径，非官方 ✓）
	// GraduationSource 毕业线**来源备查**（wiki 哪一节 / 外部面板页 URL ✓）。
	// ⚠️ 只进表与后台 ✗ —— 绝不拼进发给模型的文本 ✓（用户 2026-10-14：转载可用但不署名 ✓）
	GraduationSource string `gorm:"size:512" json:"graduationSource"`
	Notes            string `gorm:"type:text" json:"notes"` // 注意事项（命座 / 手法 / 误区）

	SourceURL   string    `gorm:"size:512" json:"sourceUrl"`
	SourceRev   string    `gorm:"size:32" json:"sourceRev"`
	FetchedAt   time.Time `json:"fetchedAt"`
	ContentHash string    `gorm:"size:64" json:"contentHash"` // 增量更新用：变了才重写 ✓

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
