package model

import (
	"time"
)

// ChatSession 会话
type ChatSession struct {
	ID     string `gorm:"primaryKey;size:36" json:"id"`
	Title  string `gorm:"size:128" json:"title"`
	UserID uint   `gorm:"index" json:"user_id"` // 0 = 匿名会话

	// 本会话是否注入过业务上下文（用于后台评估 V1 玩家档案 / V2 资产资料的效果）
	// 语义是"曾经注入过"，一旦为真不回退。
	// not null + default 是必须的：老数据 ALTER 后取默认值，否则 NULL 扫进 bool 会报错。
	HasPlayerContext  bool `gorm:"not null;default:false" json:"has_player_context"`
	HasAssetReference bool `gorm:"not null;default:false" json:"has_asset_reference"`

	// DeletedAt 软删除时间：非空表示用户已在 C 端删除该会话。
	// C 端（列表/明细/改名/续聊）一律不可见；数据保留，后台仍可查并标注「已删除」
	// （对话记录同时是审计与成本统计的来源，见 docs/04 §2.4）。
	// 用 *time.Time 而不是 gorm.DeletedAt：后者会把后台的查询也隐式过滤掉。
	DeletedAt *time.Time `gorm:"index" json:"deleted_at"`

	// UpdatedAt 建索引：后台会话列表按活跃时间倒序
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `gorm:"index" json:"updated_at"`

	Messages []ChatMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// ChatMessage 消息记录
type ChatMessage struct {
	ID        uint   `gorm:"primaryKey;autoIncrement;index:idx_chat_session_msg,priority:2" json:"id"`
	SessionID string `gorm:"index:idx_chat_session_msg,priority:1;size:36" json:"session_id"`
	Role      string `gorm:"size:16" json:"role"` // user | assistant | system
	Content   string `gorm:"type:text" json:"content"`

	// 用量与观测字段（仅 assistant 消息有意义；上游未返回则为 0/空）
	// 新增列一律 not null + default：老数据 ALTER 后取默认值，避免 NULL 扫进 bool / string 报错。
	Tokens       int    `json:"tokens"`                                     // 总 tokens
	TokensInput  int    `gorm:"not null;default:0" json:"tokens_input"`     // 输入 tokens
	TokensOutput int    `gorm:"not null;default:0" json:"tokens_output"`    // 输出 tokens
	TokensCached int    `gorm:"not null;default:0" json:"tokens_cached"`    // 命中的前缀缓存 tokens（单价约为未命中的 1/50，成本统计必须分开算）
	Model        string `gorm:"size:64;not null;default:''" json:"model"`   // 实际使用的模型
	Profile      string `gorm:"size:16;not null;default:''" json:"profile"` // 实际生效的推理档位
	Degraded     bool   `gorm:"not null;default:false" json:"degraded"`     // 是否为降级兜底回复（不要靠文案比对判断）
	ElapsedMs    int    `gorm:"not null;default:0" json:"elapsed_ms"`       // 该条回答耗时

	CreatedAt time.Time `json:"created_at"`
}
