package dto

// ChatRequest 对话请求
type ChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message" binding:"required,max=4000"`
	Stream    bool   `json:"stream"`
}

// ChatResponse 对话响应（stream=false）
type ChatResponse struct {
	SessionID string `json:"sessionId"`
	Reply     string `json:"reply"`
	Tokens    int    `json:"tokens"`
	ElapsedMs int64  `json:"elapsedMs"`
}

// 以下为 stream=true 时的 SSE 帧载荷（契约见 docs/04-API接口文档.md §2.1）

// ChatStreamSession 会话建立帧：先于任何增量下发，流中断时前端也能保留会话归属
type ChatStreamSession struct {
	SessionID string `json:"sessionId"`
}

// ChatStreamDelta 增量文本帧
type ChatStreamDelta struct {
	Text string `json:"text"`
}

// ChatStreamDone 正常结束帧（唯一终止成功帧）
type ChatStreamDone struct {
	SessionID string `json:"sessionId"`
	Tokens    int    `json:"tokens"`
	ElapsedMs int64  `json:"elapsedMs"`
}

// ChatStreamError 异常结束帧（message 已脱敏，不含上游细节）
type ChatStreamError struct {
	Message string `json:"message"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Time    string `json:"time"`
}
