package dto

// C 端会话管理（需登录，契约见 docs/04-API接口文档.md §3）
// 匿名会话（user_id = 0）不进列表，也无法通过这些接口访问。

// ChatSessionItem 会话列表项（不含消息正文，避免大 payload）
type ChatSessionItem struct {
	SessionID    string `json:"sessionId"`
	Title        string `json:"title"`
	MessageCount int64  `json:"messageCount"`
	CreatedAt    string `json:"createdAt"` // RFC3339
	UpdatedAt    string `json:"updatedAt"` // RFC3339（会话活跃时间，也是列表排序依据）
}

// ChatSessionList 会话列表（分页）
type ChatSessionList struct {
	Items    []ChatSessionItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

// ChatSessionMessageItem 会话内的一条消息
// 用量 / 降级标记属运维信息（后台 /internal/v1/admin/chat/* 可见），不下发 C 端。
type ChatSessionMessageItem struct {
	ID        uint   `json:"id"`
	Role      string `json:"role"` // user | assistant
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"` // RFC3339
}

// ChatSessionMessages 会话明细（向上游标分页：返回 id < beforeId 的最近 limit 条，按 id 升序）
type ChatSessionMessages struct {
	SessionID string                   `json:"sessionId"`
	Messages  []ChatSessionMessageItem `json:"messages"`
	HasMore   bool                     `json:"hasMore"` // 还有更早的消息（前端可继续用 beforeId 拉）
}

// RenameSessionRequest 重命名会话
type RenameSessionRequest struct {
	Title string `json:"title" binding:"required"`
}

// RenamedSession 重命名结果（返回服务端归一化后的标题）
type RenamedSession struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
}

// DeletedSession 删除结果（软删除：C 端不再可见，后台仍可查）
type DeletedSession struct {
	SessionID string `json:"sessionId"`
}
