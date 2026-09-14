package dto

// 后台运维系统的用户管理出入参（内网接口，契约见 04-用户体系与鉴权边界设计 §4）

// AdminUserQuery 用户列表查询参数
// 用量与无关问题相关参数为 2026-09-14 新增（设计见 shared-docs/05 §2.5 / §3.4）
type AdminUserQuery struct {
	Keyword string // 匹配 用户名 / 昵称 / 游戏 UID
	// UsageDate 当日用量统计的日期（YYYY-MM-DD，**本地时区**）；空 = 今天
	UsageDate string
	// MinDailyTokens 只返回 UsageDate 当天消耗 ≥ 该值的用户（5 万观察线）；0 = 不过滤
	MinDailyTokens int64
	// OnlyOffTopic 只返回有无关提问的用户
	OnlyOffTopic bool
	// OffTopicMin 无关提问数 ≥ N（OnlyOffTopic=true 时缺省按 1 处理）
	OffTopicMin int
	Page        int
	PageSize    int
}

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

	// 当日用量（UsageDate 指定，默认今天；口径见 shared-docs/05 §2.5）
	TodayTokens   int64 `json:"todayTokens"`
	TodayInput    int64 `json:"todayInput"`
	TodayOutput   int64 `json:"todayOutput"`
	TodayCached   int64 `json:"todayCached"` // 缓存命中部分单独给，便于按 ~50 倍差价折算成本
	TodayRequests int64 `json:"todayRequests"`

	// OffTopicCount 累计无关提问数（`role='user' AND off_topic=1`）；判定未开启时为 0
	OffTopicCount int64 `json:"offTopicCount"`
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
