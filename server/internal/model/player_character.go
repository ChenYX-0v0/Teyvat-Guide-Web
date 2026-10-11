package model

import "time"

// PlayerCharacter 玩家「已收录角色」（2026-10-11 新增）。
//
// ⚠️ 为什么不塞进 player_profile_snapshots ✗（那是刻意的，别合并）：
//  ① 快照是**缓存** —— TTL 10 分钟、还会被 `/api/admin/assets/purge-snapshots` **整表清空** ✗；
//     而"用户角色池"是**长期数据**，被清掉就等于用户白攒一场 ✓。
//  ② 快照按 uid **整体覆盖** ✗ —— 游戏里展柜最多 12 位 ✓，用户会**轮换**展柜；
//     覆盖式写入会让之前导入过的角色凭空消失 ✗ ⇒ 用户永远攒不齐自己的角色池 ✓。
//
// 所以这张表的口径是（用户 2026-10-11 明确要求）：
//
//	**只新增 / 只更新，永不删除** ✓ —— 用户通过反复更换展柜，
//	可以把自己的角色**逐步全部导入进来** ✓。
//
// 对应的写入规则：每轮采集 upsert 本轮出现的角色（刷新 card / lastSeenAt ✓），
// 并把**不在本轮展柜**里的行 `in_showcase` 置 false ✓（这是"标记"，不是删除 ✓）。
type PlayerCharacter struct {
	// 联合主键 (UID, CharacterID)：同一个游戏账号下，一个角色一行 ✓
	UID         string `gorm:"primaryKey;size:16" json:"uid"`
	CharacterID int64  `gorm:"primaryKey" json:"characterId"`

	// Card 最近一次采集到的角色卡（dto.CharacterCard 的 JSON ✓）。
	//
	// ⚠️ 语义是"**最后一次见到它时的样子**" ✓：角色一旦被移出展柜就不再刷新，
	// 等级 / 圣遗物 / 命座都可能已经变了 ⇒ 它**可能过时** ✓。
	// 因此界面上必须展示 LastSeenAt ✓（别让人以为这是实时数据 ✗）。
	Card string `gorm:"type:json" json:"card"`

	// InShowcase 当前是否在展柜里（**标记位，不是删除** ✓）。
	// 每轮采集：本轮出现的置 true ✓、其余置 false ✓ —— 历史角色仍然完整保留 ✓。
	InShowcase bool `gorm:"index" json:"inShowcase"`

	// FirstSeenAt 首次收录时间（**upsert 时刻意不覆盖** ✓ —— 它记录"这个角色是什么时候进池的" ✓）
	FirstSeenAt time.Time `json:"firstSeenAt"`
	// LastSeenAt 最近一次"在展柜里见到它"的时间 = 这张 card 的新鲜度 ✓
	LastSeenAt time.Time `json:"lastSeenAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
