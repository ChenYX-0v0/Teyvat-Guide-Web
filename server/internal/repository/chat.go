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

// SessionRow 会话列表行（含消息数聚合）
type SessionRow struct {
	ID           string
	Title        string
	MessageCount int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ListSessionsByUser 某用户的会话列表（分页）。
// 只查 user_id = userID 且**未删除**的会话——匿名会话（user_id = 0）不进任何人的列表；
// 排序按活跃时间倒序 + id 兜底（created_at/updated_at 精度为秒，同秒必须靠 id 定序）。
func (r *ChatRepository) ListSessionsByUser(userID uint, offset, limit int) ([]SessionRow, int64, error) {
	if r.db() == nil {
		return nil, 0, nil
	}

	base := func() *gorm.DB {
		return r.db().Table("chat_sessions AS s").
			Joins("LEFT JOIN (" + chatAggregateSQL + ") AS a ON a.session_id = s.id").
			Where("s.user_id = ?", userID).
			Where("s.deleted_at IS NULL")
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []SessionRow
	err := base().
		Select(`s.id AS id, s.title AS title,
			COALESCE(a.message_count, 0) AS message_count,
			s.created_at AS created_at, s.updated_at AS updated_at`).
		Order("s.updated_at DESC, s.id DESC").
		Offset(offset).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// RenameSession 重命名会话（仅本人：同时以 id 与 user_id 过滤；返回是否命中一行）。
// 用 UpdateColumn 而非 Update：避免 GORM 自动刷新 updated_at——
// 重命名不该改变"会话活跃时间"，否则列表顺序会因改名而跳动。
func (r *ChatRepository) RenameSession(sessionID string, userID uint, title string) (bool, error) {
	if r.db() == nil {
		return false, nil
	}
	res := r.db().Model(&model.ChatSession{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", sessionID, userID).
		UpdateColumn("title", title)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// SoftDeleteSession 软删除会话（仅本人；只置 deleted_at，不物理删除）。
// C 端从此不可见（列表/明细/改名/续聊都会被过滤），后台仍可查（审计与成本统计）。
// 返回是否命中：已删除的会话再次调用不会命中 → 上层统一按 404 处理（对 C 端就是"不存在"）。
func (r *ChatRepository) SoftDeleteSession(sessionID string, userID uint) (bool, error) {
	if r.db() == nil {
		return false, nil
	}
	res := r.db().Model(&model.ChatSession{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", sessionID, userID).
		UpdateColumn("deleted_at", time.Now())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
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

// MarkSessionContext 标记会话曾注入过业务上下文（一旦为真不回退），供后台观测 V1/V2 命中情况
func (r *ChatRepository) MarkSessionContext(sessionID string, hasPlayer, hasAsset bool) error {
	if r.db() == nil {
		return nil
	}
	updates := map[string]any{}
	if hasPlayer {
		updates["has_player_context"] = true
	}
	if hasAsset {
		updates["has_asset_reference"] = true
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db().Model(&model.ChatSession{}).Where("id = ?", sessionID).Updates(updates).Error
}

// ---------- 后台对话记录查看（只读；契约见 admin/06-对话记录查看设计.md） ----------

// chatAggregateSQL 会话级聚合（消息数 / token / 降级条数）；方言中立写法
const chatAggregateSQL = `
SELECT session_id,
       COUNT(*) AS message_count,
       COALESCE(SUM(tokens), 0) AS tokens,
       COALESCE(SUM(CASE WHEN degraded = 1 THEN 1 ELSE 0 END), 0) AS degraded_count
FROM chat_messages
GROUP BY session_id`

// AdminSessionFilter 后台会话列表的查询条件
type AdminSessionFilter struct {
	Keyword      string     // 只匹配「提问」内容（role = user）
	UserID       uint       // 0 表示不过滤
	UID          string     // 按绑定的游戏 UID 过滤
	OnlyDegraded bool       // 只看含降级回复的会话
	OnlyDeleted  bool       // 只看用户已在 C 端删除的会话
	From         *time.Time // 按会话活跃时间过滤
	To           *time.Time
	Offset       int
	Limit        int
}

// AdminSessionRow 后台会话列表行（会话 + 归属用户 + 会话级聚合）
type AdminSessionRow struct {
	ID                string
	Title             string
	UserID            uint
	Username          string
	DisplayName       string
	GenshinUID        string
	HasPlayerContext  bool
	HasAssetReference bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time // 用户 C 端删除时间；nil = 未删除
	MessageCount      int64
	Tokens            int64
	DegradedCount     int64
}

// ListAdminSessions 后台会话列表（排序：活跃时间倒序 + id 兜底，避免同秒乱序）。
// **不过滤已软删除的会话**：用户删掉的对话对运营仍要可见（审计与成本统计），
// 仅在结果里带出 DeletedAt 由前端标注；OnlyDeleted=true 时只看这些。
func (r *ChatRepository) ListAdminSessions(f AdminSessionFilter) ([]AdminSessionRow, int64, error) {
	if r.db() == nil {
		return nil, 0, nil
	}

	base := func() *gorm.DB {
		q := r.db().Table("chat_sessions AS s").
			Joins("LEFT JOIN users AS u ON u.id = s.user_id").
			Joins("LEFT JOIN (" + chatAggregateSQL + ") AS a ON a.session_id = s.id")
		if f.Keyword != "" {
			// 既定口径：关键词只搜提问
			q = q.Where(
				"EXISTS (SELECT 1 FROM chat_messages AS km WHERE km.session_id = s.id AND km.role = 'user' AND km.content LIKE ?)",
				"%"+f.Keyword+"%",
			)
		}
		if f.UserID > 0 {
			q = q.Where("s.user_id = ?", f.UserID)
		}
		if f.UID != "" {
			q = q.Where("u.genshin_uid = ?", f.UID)
		}
		if f.OnlyDegraded {
			q = q.Where("COALESCE(a.degraded_count, 0) > 0")
		}
		if f.OnlyDeleted {
			q = q.Where("s.deleted_at IS NOT NULL")
		}
		if f.From != nil {
			q = q.Where("s.updated_at >= ?", *f.From)
		}
		if f.To != nil {
			q = q.Where("s.updated_at <= ?", *f.To)
		}
		return q
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []AdminSessionRow
	err := base().
		Select(`s.id AS id, s.title AS title, s.user_id AS user_id,
			COALESCE(u.username, '') AS username, COALESCE(u.display_name, '') AS display_name,
			COALESCE(u.genshin_uid, '') AS genshin_uid,
			s.has_player_context AS has_player_context, s.has_asset_reference AS has_asset_reference,
			s.created_at AS created_at, s.updated_at AS updated_at, s.deleted_at AS deleted_at,
			COALESCE(a.message_count, 0) AS message_count, COALESCE(a.tokens, 0) AS tokens,
			COALESCE(a.degraded_count, 0) AS degraded_count`).
		Order("s.updated_at DESC, s.id DESC").
		Offset(f.Offset).
		Limit(f.Limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListLastQuestions 批量取每个会话最近一条「提问」（列表摘要用，避免整段正文进列表）
func (r *ChatRepository) ListLastQuestions(sessionIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(sessionIDs))
	if r.db() == nil || len(sessionIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		SessionID string
		Content   string
	}
	err := r.db().Table("chat_messages AS m").
		Select("m.session_id AS session_id, m.content AS content").
		Joins(`JOIN (SELECT session_id, MAX(id) AS max_id FROM chat_messages
			WHERE role = 'user' AND session_id IN ? GROUP BY session_id) AS t
			ON t.session_id = m.session_id AND t.max_id = m.id`, sessionIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SessionID] = row.Content
	}
	return out, nil
}

// FindSession 按 ID 取会话（不存在返回 nil, nil）
func (r *ChatRepository) FindSession(sessionID string) (*model.ChatSession, error) {
	if r.db() == nil {
		return nil, nil
	}
	var s model.ChatSession
	err := r.db().First(&s, "id = ?", sessionID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// ListMessagesAfter 会话内消息（游标分页：id > afterID，升序返回）
func (r *ChatRepository) ListMessagesAfter(sessionID string, afterID uint, limit int) ([]model.ChatMessage, error) {
	if r.db() == nil {
		return nil, nil
	}
	q := r.db().Where("session_id = ?", sessionID)
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	var list []model.ChatMessage
	err := q.Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}

// ListMessagesBefore 取会话内 id < beforeID 的**最近** limit 条消息（beforeID = 0 表示不设上界），
// 返回按 id 升序排列的结果。C 端打开会话先看最新一屏、再往前翻历史，因此游标是 beforeId。
func (r *ChatRepository) ListMessagesBefore(sessionID string, beforeID uint, limit int) ([]model.ChatMessage, error) {
	if r.db() == nil {
		return nil, nil
	}
	q := r.db().Where("session_id = ?", sessionID)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	var list []model.ChatMessage
	if err := q.Order("id desc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	// 反转为升序：会话视图按时间正序渲染
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
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
