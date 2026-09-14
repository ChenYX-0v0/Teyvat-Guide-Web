package model

import "time"

// PresetQA 预设问答：匿名用户在对话页看到的内容。
// 匿名已被禁用 AI 对话（`/api/chat` 强制登录，口径见 shared-docs/05 §1），
// 因此这条链路**不调用模型、不消耗 token、不落库会话**，只是把运营准备好的问答发出去。
type PresetQA struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	SceneKey string `gorm:"size:32;not null;default:'';index" json:"scene_key"` // 分组，如 onboarding / build / explore
	Question string `gorm:"size:255;not null" json:"question"`                  // 预设提问（展示用）
	Answer   string `gorm:"type:text;not null" json:"answer"`                   // 预设回答（纯文本，前端按纯文本渲染）
	Sort     int    `gorm:"not null;default:0" json:"sort"`                     // 组内排序
	Enabled  bool   `gorm:"not null;default:true" json:"enabled"`               // 下架开关；不做物理删除

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
