package dto

// ChatRequest 对话请求
type ChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message" binding:"required,max=4000"`
	Stream    bool   `json:"stream"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	SessionID string `json:"sessionId"`
	Reply     string `json:"reply"`
	Tokens    int    `json:"tokens"`
	ElapsedMs int64  `json:"elapsedMs"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Time    string `json:"time"`
}
