package model

import "time"

// AIPrompt AI 提示词（业务数据入库，禁止硬编码；AI 服务只读本表）
//
// 场景键分层（paimon_base → paimon_rules → paimon_fewshot）决定前缀缓存的稳定性，
// 顺序一旦上线不要调整，否则 DeepSeek 前缀缓存会整段失效（命中价差约 50 倍）。
//
// 同一 scene_key 可以有多个 version：对外只暴露「启用且版本最高」的一条，
// 便于灰度与回滚（旧版本留着，回滚只需把新版本 enabled 置 0）。
//
// 表结构与种子数据见 AI 服务仓 `db/migrations/001_ai_prompts.sql`；
// 读取走 `GET /internal/v1/ai/prompts`（契约见 shared-docs/03 §5）。
type AIPrompt struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	SceneKey string `gorm:"size:64;not null;uniqueIndex:uk_scene_version,priority:1" json:"sceneKey"`
	Content  string `gorm:"type:text;not null" json:"content"`
	Version  int    `gorm:"not null;default:1;uniqueIndex:uk_scene_version,priority:2" json:"version"`
	Enabled  bool   `gorm:"not null;default:true" json:"enabled"`
	Remark   string `gorm:"size:255;not null;default:''" json:"remark"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 固定表名，避免 GORM 对连续大写（AI）的拆词差异导致建出第二张表
func (AIPrompt) TableName() string { return "ai_prompts" }
