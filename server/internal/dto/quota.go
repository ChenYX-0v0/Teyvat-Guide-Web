package dto

// QuotaExceeded 配额 / 并发超限时的 data（契约见 docs/04-API接口文档.md §2.6）。
// 日配额：Used/Limit 为 token；并发：Used/Limit 为在途请求数（ResetAt 为空）。
type QuotaExceeded struct {
	Reason  string `json:"reason"` // daily | concurrent | global
	Used    int64  `json:"used"`
	Limit   int64  `json:"limit"`
	ResetAt string `json:"resetAt,omitempty"`
}
