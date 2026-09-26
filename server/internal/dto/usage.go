package dto

// UsageEventInput 单条埋点事件（前端 → `POST /api/events`，契约见 shared-docs/07 §4.1/§5）
type UsageEventInput struct {
	Event     string         `json:"event" binding:"required,max=64"`
	Props     map[string]any `json:"props,omitempty"`
	SessionID string         `json:"sessionId,omitempty" binding:"max=64"`
	Page      string         `json:"page,omitempty" binding:"max=32"`
	Platform  string         `json:"platform,omitempty" binding:"max=8"`
	ClientTS  int64          `json:"clientTs,omitempty"`
}

// UsageIngestRequest 批量提交：单请求最多 20 条（超过直接整批忽略，前端本来也不该这么发）
type UsageIngestRequest struct {
	Events []UsageEventInput `json:"events" binding:"required,min=1,max=20,dive"`
}
