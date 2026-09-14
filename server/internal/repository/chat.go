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

// ---------- 后台：用量与无关问题观测（见 shared-docs/05 §2.5 / §3.4） ----------

// UserUsageRow 某用户在一个时间窗内的用量聚合
type UserUsageRow struct {
	UserID       uint
	Tokens       int64
	TokensInput  int64
	TokensOutput int64
	TokensCached int64
	Requests     int64 // assistant 消息数（= 回答条数）
}

// SumUsageByUsersInRange 批量取若干用户在 [from, to) 内的用量（按用户分组）。
// 时间窗用左闭右开 + 半开区间，避免跨天边界重复计数。
func (r *ChatRepository) SumUsageByUsersInRange(userIDs []uint, from, to time.Time) (map[uint]UserUsageRow, error) {
	out := make(map[uint]UserUsageRow, len(userIDs))
	if r.db() == nil || len(userIDs) == 0 {
		return out, nil
	}
	var rows []UserUsageRow
	err := r.db().Table("chat_messages AS m").
		Select(`s.user_id AS user_id,
			COALESCE(SUM(m.tokens), 0) AS tokens,
			COALESCE(SUM(m.tokens_input), 0) AS tokens_input,
			COALESCE(SUM(m.tokens_output), 0) AS tokens_output,
			COALESCE(SUM(m.tokens_cached), 0) AS tokens_cached,
			COALESCE(SUM(CASE WHEN m.role = 'assistant' THEN 1 ELSE 0 END), 0) AS requests`).
		Joins("JOIN chat_sessions AS s ON s.id = m.session_id").
		Where("s.user_id IN ? AND m.created_at >= ? AND m.created_at < ?", userIDs, from, to).
		Group("s.user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = row
	}
	return out, nil
}

// SumOffTopicByUsers 批量取若干用户的**累计**无关提问数（`role='user' AND off_topic=1`；NULL 不计数）
func (r *ChatRepository) SumOffTopicByUsers(userIDs []uint) (map[uint]int64, error) {
	out := make(map[uint]int64, len(userIDs))
	if r.db() == nil || len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID uint
		Total  int64
	}
	err := r.db().Table("chat_messages AS m").
		Select("s.user_id AS user_id, COUNT(*) AS total").
		Joins("JOIN chat_sessions AS s ON s.id = m.session_id").
		Where("s.user_id IN ? AND m.role = 'user' AND m.off_topic = 1", userIDs).
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

// UserIDsWithTokensAtLeast 取 [from, to) 内用量 ≥ minTokens 的用户 ID（后台「5 万观察线」筛选用）。
// 返回的切片**非 nil**（可能为空）：调用方据此走"筛出来是空集"分支，而不是"不过滤"。
func (r *ChatRepository) UserIDsWithTokensAtLeast(from, to time.Time, minTokens int64) ([]uint, error) {
	out := make([]uint, 0)
	if r.db() == nil {
		return out, nil
	}
	var rows []struct{ UserID uint }
	err := r.db().Table("chat_messages AS m").
		Select("s.user_id AS user_id").
		Joins("JOIN chat_sessions AS s ON s.id = m.session_id").
		Where("s.user_id > 0 AND m.created_at >= ? AND m.created_at < ?", from, to).
		Group("s.user_id").
		Having("COALESCE(SUM(m.tokens), 0) >= ?", minTokens).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, row.UserID)
	}
	return out, nil
}

// UserIDsWithOffTopic 取累计无关提问数 ≥ minCount 的用户 ID（minCount < 1 时按 1 处理）
func (r *ChatRepository) UserIDsWithOffTopic(minCount int64) ([]uint, error) {
	out := make([]uint, 0)
	if r.db() == nil {
		return out, nil
	}
	if minCount < 1 {
		minCount = 1
	}
	var rows []struct{ UserID uint }
	err := r.db().Table("chat_messages AS m").
		Select("s.user_id AS user_id").
		Joins("JOIN chat_sessions AS s ON s.id = m.session_id").
		Where("s.user_id > 0 AND m.role = 'user' AND m.off_topic = 1").
		Group("s.user_id").
		Having("COUNT(*) >= ?", minCount).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, row.UserID)
	}
	return out, nil
}

// SumUserTokensSince 某用户自 since（含）起的 token 消耗。
// 配额口径：`chat_messages.tokens` 经 `chat_sessions.user_id` 归属，与后台成本统计同源
// （见 shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md §2.1）。
func (r *ChatRepository) SumUserTokensSince(userID uint, since time.Time) (int64, error) {
	if r.db() == nil || userID == 0 {
		return 0, nil
	}
	var total int64
	err := r.db().Table("chat_messages AS m").
		Select("COALESCE(SUM(m.tokens), 0)").
		Joins("JOIN chat_sessions AS s ON s.id = m.session_id").
		Where("s.user_id = ? AND m.created_at >= ?", userID, since).
		Row().Scan(&total)
	return total, err
}

// SumTotalTokensSince 全站自 since（含）起的 token 消耗（全局日预算熔断用）
func (r *ChatRepository) SumTotalTokensSince(since time.Time) (int64, error) {
	if r.db() == nil {
		return 0, nil
	}
	var total int64
	err := r.db().Table("chat_messages").
		Select("COALESCE(SUM(tokens), 0)").
		Where("created_at >= ?", since).
		Row().Scan(&total)
	return total, err
}

// MarkMessageOffTopic 记录无关问题判定结果。
// offTopic 为 nil 表示**未判定**：直接跳过，保持列为 NULL（不要写成 false）。
func (r *ChatRepository) MarkMessageOffTopic(messageID uint, offTopic *bool, reason string) error {
	if r.db() == nil || messageID == 0 || offTopic == nil {
		return nil
	}
	return r.db().Model(&model.ChatMessage{}).
		Where("id = ?", messageID).
		Updates(map[string]any{
			"off_topic":        *offTopic,
			"off_topic_reason": reason,
		}).Error
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

// chatAggregateSQL 会话级聚合（消息数 / token / 降级条数 / 无关提问条数）；方言中立写法。
// off_topic 为 NULL（未判定）时不计数——判定未开启时该列恒为 0，不要与"判定为有关"混淆。
const chatAggregateSQL = `
SELECT session_id,
       COUNT(*) AS message_count,
       COALESCE(SUM(tokens), 0) AS tokens,
       COALESCE(SUM(CASE WHEN degraded = 1 THEN 1 ELSE 0 END), 0) AS degraded_count,
       COALESCE(SUM(CASE WHEN role = 'user' AND off_topic = 1 THEN 1 ELSE 0 END), 0) AS off_topic_count
FROM chat_messages
GROUP BY session_id`

// AdminSessionFilter 后台会话列表的查询条件
type AdminSessionFilter struct {
	Keyword      string     // 只匹配「提问」内容（role = user）
	UserID       uint       // 0 表示不过滤
	UID          string     // 按绑定的游戏 UID 过滤
	OnlyDegraded bool       // 只看含降级回复的会话
	OnlyDeleted  bool       // 只看用户已在 C 端删除的会话
	OnlyOffTopic bool       // 只看**含无关提问**的会话（见 shared-docs/05 §3.4）
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
	OffTopicCount     int64 // 会话内无关提问条数（判定未开启时为 0）
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
		if f.OnlyOffTopic {
			// 含无关提问的会话（判定未开启时恒不命中，符合预期）
			q = q.Where("COALESCE(a.off_topic_count, 0) > 0")
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
			COALESCE(a.degraded_count, 0) AS degraded_count,
			COALESCE(a.off_topic_count, 0) AS off_topic_count`).
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
