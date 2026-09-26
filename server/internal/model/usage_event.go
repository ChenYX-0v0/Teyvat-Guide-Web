package model

import "time"

// UsageEvent 埋点事件明细（真源：shared-docs/07-前端埋点与使用统计.md §3.1）
//
// 只存「服务端本来就看不见」的前端行为：页面浏览与停留、入口点击、空态与失败、前端错误。
// ⚠️ 提问数 / token 成本 / 无关问题占比等已有落库数据（chat_messages 等）**不要在这里重复采集**
// （§2「先看哪些指标能白拿」是这套东西的核心取舍）。
//
// ⚠️ D1 定案（2026-09-26）：**不采集匿名标识** —— user_id 为空即匿名。
// 匿名行为只统计"次数"，不做匿名去重、不做"匿名→登录"归因，因此本表**刻意没有 anon_id 字段**。
//
// ⚠️ 统计一律以 created_at（服务端时间）为准，client_ts 仅作参考（防客户端改时间）。
type UsageEvent struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Event        string    `gorm:"size:64;not null;index:idx_usage_event_time,priority:1" json:"event"`
	EventVersion uint16    `gorm:"not null;default:1" json:"eventVersion"`
	UserID       *uint     `gorm:"index:idx_usage_user_time,priority:1" json:"userId,omitempty"` // NULL = 匿名
	SessionID    string    `gorm:"size:64;not null;default:''" json:"sessionId,omitempty"`
	Page         string    `gorm:"size:32;not null;default:''" json:"page,omitempty"`
	Platform     string    `gorm:"size:8;not null;default:''" json:"platform,omitempty"`
	Props        string    `gorm:"type:json" json:"props,omitempty"`
	ClientTS     int64     `gorm:"not null;default:0" json:"clientTs,omitempty"`
	CreatedAt    time.Time `gorm:"index:idx_usage_event_time,priority:2;index:idx_usage_created" json:"createdAt"`
}

// TableName 固定表名，避免 GORM 拆词差异
func (UsageEvent) TableName() string { return "usage_events" }
