package model

import "time"

// InviteCode 邀请码（2026-09-28 新增）—— 注册的第二道门（第一道是短信验证码 ✓）
//
// 口径（用户 2026-09-28 定 ✓）：
//   - 注册表单里**邀请码在手机号上方**；
//   - **不填不能发短信**、**码不对也不能发短信**；
//   - **一次性**：用过一次即作废 ✓。
//
// ⚠️ 为什么"作废"发生在**注册成功**那一刻，而不是"发短信"时 ✗：
// 发完码用户可能填错短信码 / 直接放弃 ⇒ 码就被白白烧掉 ✗。
// 所以发码阶段只**校验**（不占用 ✓），真正占用在 `repository.CreateUserWithInvite` 的
// 同一个事务里原子完成 ✓（占用失败则连用户一起回滚 ✓✓）。
type InviteCode struct {
	// Code 邀请码本体（主键 ⇒ 天然唯一 ✓）；归一化后存大写，见 service.NormalizeInviteCode
	Code string `gorm:"size:32;primaryKey" json:"code"`
	// Note 备注（发给谁 / 什么渠道，便于对账与回收）
	Note string `gorm:"size:64" json:"note,omitempty"`
	// Enabled 是否可用；false = 停用（发错了、要收回时用，**不删行**以便留痕 ✓）
	Enabled bool `gorm:"not null;default:true" json:"enabled"`
	// UsedAt 使用时间（**为空 = 未使用**，这是"是否作废"的唯一判据 ✓）
	UsedAt *time.Time `json:"usedAt,omitempty"`
	// UsedBy 用它注册出来的用户 ID（留痕，便于排查"这个号是谁放进来的"）
	UsedBy *uint `json:"usedBy,omitempty"`
	// UsedPhone 使用时填的手机号（脱敏前的原值；后台只读）
	UsedPhone string    `gorm:"size:16" json:"usedPhone,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Used 是否已被使用（= 已作废）
func (i *InviteCode) Used() bool { return i.UsedAt != nil }

// Usable 当前是否可用（启用 且 未使用）
func (i *InviteCode) Usable() bool { return i.Enabled && !i.Used() }
