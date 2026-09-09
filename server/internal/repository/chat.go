package repository

import (
	"time"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
)

// ChatRepository 会话与消息持久化
type ChatRepository struct{}

func NewChatRepository() *ChatRepository {
	return &ChatRepository{}
}

func (r *ChatRepository) db() *gorm.DB {
	return database.DB()
}

// CreateSession 新建会话
func (r *ChatRepository) CreateSession(s *model.ChatSession) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Create(s).Error
}

// AppendMessage 追加一条消息
func (r *ChatRepository) AppendMessage(m *model.ChatMessage) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Create(m).Error
}

// ListMessages 查询会话历史（按时间正序，最多 limit 条）
func (r *ChatRepository) ListMessages(sessionID string, limit int) ([]model.ChatMessage, error) {
	if r.db() == nil {
		return nil, nil
	}
	var list []model.ChatMessage
	err := r.db().
		Where("session_id = ?", sessionID).
		Order("created_at asc").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// Touch 更新会话活跃时间
func (r *ChatRepository) Touch(sessionID string) error {
	if r.db() == nil {
		return nil
	}
	return r.db().
		Model(&model.ChatSession{}).
		Where("id = ?", sessionID).
		Update("updated_at", time.Now()).Error
}
