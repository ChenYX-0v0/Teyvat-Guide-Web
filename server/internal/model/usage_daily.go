package model

// UsageDaily 埋点按天聚合（真源：shared-docs/07-前端埋点与使用统计.md §3.2）
//
// 看板的排行榜/趋势读它，不扫明细表。聚合任务每小时重建「今天 + 昨天」两天的行（幂等：
// 先按天 DELETE 再 INSERT），所以当天数据最多滞后一小时，而不是等次日 03:30。
//
// ⚠️ D1 定案：**不采集匿名标识** → 匿名事件只有 PV；`uv` 只统计登录用户（`COUNT(DISTINCT user_id)`）。
// 因此**跨事件把 uv 相加是错的**（同一人会在多个事件里重复），看板上的"总 UV"一律回明细表算。
type UsageDaily struct {
	Day      string `gorm:"size:10;primaryKey" json:"day"` // YYYY-MM-DD（UTC+8，与库内 created_at 同时区）
	Event    string `gorm:"size:64;primaryKey" json:"event"`
	DimKey   string `gorm:"size:16;primaryKey" json:"dimKey"` // "" | page | platform
	DimValue string `gorm:"size:32;primaryKey" json:"dimValue"`
	PV       int64  `gorm:"not null;default:0" json:"pv"`
	UV       int64  `gorm:"not null;default:0" json:"uv"` // 登录用户去重
}

func (UsageDaily) TableName() string { return "usage_daily" }
