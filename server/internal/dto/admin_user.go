package dto

// 后台运维系统的用户管理出入参（内网接口，契约见 04-用户体系与鉴权边界设计 §4）

// AdminUserItem 用户列表项
type AdminUserItem struct {
	ID          uint    `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	GenshinUID  *string `json:"genshinUid"`  // 未绑定为 null
	Status      string  `json:"status"`      // active | banned
	CreatedAt   string  `json:"createdAt"`   // RFC3339
	LastLoginAt *string `json:"lastLoginAt"` // 从未登录为 null
	TotalTokens int64   `json:"totalTokens"` // 该用户累计 token 消耗
}

// AdminUserList 用户列表（分页）
type AdminUserList struct {
	Items    []AdminUserItem `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

// SetUserStatusRequest 封禁 / 解封请求
type SetUserStatusRequest struct {
	ID     uint `json:"id" binding:"required"`
	Banned bool `json:"banned"`
}
