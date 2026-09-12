package dto

// 后台对话记录查看（内网只读接口，契约见 admin/06-对话记录查看设计.md §4）

// AdminChatQuery 会话列表查询参数
type AdminChatQuery struct {
	Keyword      string // 只匹配提问内容
	UserID       uint   // 0 = 不过滤
	UID          string
	OnlyDegraded bool
	OnlyDeleted  bool   // 只看用户已在 C 端删除的会话（后台仍可查，见 docs/04 §2.4）
	From         string // RFC3339
	To           string // RFC3339
	Page         int
	PageSize     int
}

// AdminChatSessionItem 会话列表项（不含消息正文，避免大 payload）
type AdminChatSessionItem struct {
	SessionID         string  `json:"sessionId"`
	Title             string  `json:"title"`
	UserID            uint    `json:"userId"` // 0 = 匿名
	Username          *string `json:"username"`
	DisplayName       *string `json:"displayName"`
	GenshinUID        *string `json:"genshinUid"`
	MessageCount      int64   `json:"messageCount"`
	Tokens            int64   `json:"tokens"`
	DegradedCount     int64   `json:"degradedCount"`
	HasPlayerContext  bool    `json:"hasPlayerContext"`
	HasAssetReference bool    `json:"hasAssetReference"`
	LastQuestion      *string `json:"lastQuestion"` // 截断摘要
	CreatedAt         string  `json:"createdAt"`    // RFC3339
	UpdatedAt         string  `json:"updatedAt"`    // RFC3339
	// DeletedAt 用户 C 端删除时间；null = 未删除（后台默认不过滤已删除会话，只做标注）
	DeletedAt *string `json:"deletedAt"`
}

// AdminChatSessionList 会话列表（分页）
type AdminChatSessionList struct {
	Items    []AdminChatSessionItem `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}

// AdminChatMessageItem 会话内的一条消息
type AdminChatMessageItem struct {
	ID           uint   `json:"id"`
	Role         string `json:"role"` // user | assistant
	Content      string `json:"content"`
	Tokens       int    `json:"tokens"`
	TokensInput  int    `json:"tokensInput"`
	TokensOutput int    `json:"tokensOutput"`
	TokensCached int    `json:"tokensCached"`
	Model        string `json:"model"`
	Profile      string `json:"profile"`
	Degraded     bool   `json:"degraded"`
	ElapsedMs    int    `json:"elapsedMs"`
	CreatedAt    string `json:"createdAt"`
}

// AdminChatMessages 会话明细（游标分页）
type AdminChatMessages struct {
	SessionID string                 `json:"sessionId"`
	UserID    uint                   `json:"userId"`
	Messages  []AdminChatMessageItem `json:"messages"`
	HasMore   bool                   `json:"hasMore"`
}
