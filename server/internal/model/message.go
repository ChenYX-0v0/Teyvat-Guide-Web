package model

import (
	"time"
)

// ChatSession 会话
type ChatSession struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Title     string    `gorm:"size:128" json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Messages []ChatMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// ChatMessage 消息记录
type ChatMessage struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID string    `gorm:"index;size:36" json:"session_id"`
	Role      string    `gorm:"size:16" json:"role"` // user | assistant | system
	Content   string    `gorm:"type:text" json:"content"`
	Tokens    int       `json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
}
