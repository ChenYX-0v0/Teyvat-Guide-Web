package dto

// PlayerContextStatus 聊天页用的「派蒙能不能看到你的角色」状态（2026-10-12，配合聊天兜底 ✓）。
//
// 为什么要有它：此前"聊天没有角色信息"有**三条不同原因**（未绑定 UID / 上游查询失败 / 展柜为空）✓，
// 但对用户是**同一个结果** ✗、且没有任何提示 ✓ —— 用户报"取不到角色信息"时连自己都判断不出是哪一条 ✗。
// 这个状态让前端**能说清原因** ✓（并给出下一步：去账号页绑定 / 换回展柜 ✓）。
//
// ⚠️ 它**只读本地**（绑定 + 快照 + 已收录角色池 ✓），**绝不查上游** ✗ —— 前端每次进聊天页都会调它，
// 不能让它变成一次 Enka 请求 ✗（慢、且会占上游配额 ✓）。
type PlayerContextStatus struct {
	// Bound 是否已绑定游戏 UID
	Bound bool `json:"bound"`
	// UID 绑定的 UID（未绑定时为空串）
	UID string `json:"uid,omitempty"`
	// Ready 聊天**是否会带上角色信息**：= 已绑定 且（有本地快照 或 已收录角色池非空 ✓）
	// （上游临时失败不算 false ✓ —— 有池子就能兜底 ✓）
	Ready bool `json:"ready"`
	// Reason 不能带上的原因码："" | "not_bound" | "no_profile"
	Reason string `json:"reason,omitempty"`
	// HasSnapshot 本地是否有该 UID 的档案快照（快照 TTL 过期后为 false，会触发重新拉取 ✓）
	HasSnapshot bool `json:"hasSnapshot"`
	// CollectedCount 已收录角色数（兜底用得上它 ✓；0 表示连历史收录都没有 ✓）
	CollectedCount int `json:"collectedCount"`
}
