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

// ListRecentMessages 查询会话**最近** limit 条消息，返回按时间正序排列的结果。
// 注意：不能用 `ORDER BY created_at ASC LIMIT n`——那取到的是最旧的 n 条，
// 长会话会永远停在开头；id 作为同秒内的次序兜底（created_at 精度为秒）。
func (r *ChatRepository) ListRecentMessages(sessionID string, limit int) ([]model.ChatMessage, error) {
	if r.db() == nil {
		return nil, nil
	}
	var list []model.ChatMessage
	err := r.db().
		Where("session_id = ?", sessionID).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	// 反转为时间正序：模型需要正序上下文
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// SumTokensByUsers 按用户聚合 token 消耗（chat_messages.tokens 经 chat_sessions.user_id 归属）
func (r *ChatRepository) SumTokensByUsers(userIDs []uint) (map[uint]int64, error) {
	out := make(map[uint]int64, len(userIDs))
	if r.db() == nil || len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID uint
		Total  int64
	}
	err := r.db().Table("chat_messages AS m").
		Select("s.user_id AS user_id, COALESCE(SUM(m.tokens), 0) AS total").
		Joins("JOIN chat_sessions AS s ON s.id = m.session_id").
		Where("s.user_id IN ?", userIDs).
		Group("s.user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = row.Total
	}
	return out, nil
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
