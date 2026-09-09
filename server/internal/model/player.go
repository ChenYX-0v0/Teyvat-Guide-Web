package model

import "time"

// PlayerProfileSnapshot 玩家档案快照
// 主键为游戏 UID；查询结果整体以 JSON 存于 profile 字段
type PlayerProfileSnapshot struct {
	UID       string    `gorm:"primaryKey;size:16" json:"uid"`
	Profile   string    `gorm:"type:json" json:"profile"` // dto.PlayerProfile 的 JSON 序列化
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
