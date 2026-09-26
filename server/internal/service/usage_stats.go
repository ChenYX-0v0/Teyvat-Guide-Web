package service

// 埋点统计：按天聚合 + 看板查询（真源：shared-docs/07-前端埋点与使用统计.md §3.2 / §6）
//
// 两个口径上的坑，改这里之前先读：
//  1. **uv 不能跨事件相加** —— D1 不采集匿名标识后，uv 只统计登录用户；同一人会在多个事件里
//     各算一次。看板的"总 UV"永远回明细表 `COUNT(DISTINCT user_id)` 算（见 Overview）。
//  2. 明细只保留 90 天，聚合表永久 —— 所以**长区间（>90 天）的 PV 看聚合表，UV 只能给 90 天内**。

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/pkg/logger"
)

// ── 聚合 ──

// aggLoop 每小时重建「今天 + 昨天」的聚合行（幂等；当天数据最多滞后 1 小时）
func (s *UsageService) aggLoop() {
	// 启动时先补一次，避免刚部署完看板是空的
	s.rebuildDays(2)
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for range tick.C {
		s.rebuildDays(2)
	}
}

func (s *UsageService) rebuildDays(days int) {
	db := database.DB()
	if db == nil {
		return
	}
	for i := 0; i < days; i++ {
		day := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if err := s.rebuildDay(day); err != nil {
			logger.L().Warnw("usage daily rebuild failed", "day", day, "err", err)
		}
	}
}

// rebuildDay 重算某一天的聚合行：先删该天，再按 总计 / page / platform 三个维度插入
func (s *UsageService) rebuildDay(day string) error {
	db := database.DB()
	if db == nil {
		return nil
	}

	if err := db.Exec("DELETE FROM usage_daily WHERE day = ?", day).Error; err != nil {
		return err
	}

	// ① 总计（dim_key='' ）② page 维度 ③ platform 维度
	//
	// ⚠️ 两处踩过坑的写法（2026-09-26 RDS 实测）：
	//  1. day 用**字面量参数** `?` 传，而不是 `SELECT DATE(created_at)` —— 后者在 SELECT 里出现
	//     但不在 GROUP BY 里，会被 `only_full_group_by` 直接拒（Error 1055），聚合静默失败、
	//     排行榜空着；
	//  2. 过滤用 `created_at` 的**区间**（可走索引），别用 `DATE(created_at) = ?`（函数作用在列上，
	//     用不上索引）。
	stmts := []string{
		`INSERT INTO usage_daily (day, event, dim_key, dim_value, pv, uv)
		 SELECT ?, event, '', '', COUNT(*), COUNT(DISTINCT user_id)
		 FROM usage_events WHERE created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)
		 GROUP BY event`,
		`INSERT INTO usage_daily (day, event, dim_key, dim_value, pv, uv)
		 SELECT ?, event, 'page', page, COUNT(*), COUNT(DISTINCT user_id)
		 FROM usage_events WHERE created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY) AND page <> ''
		 GROUP BY event, page`,
		`INSERT INTO usage_daily (day, event, dim_key, dim_value, pv, uv)
		 SELECT ?, event, 'platform', platform, COUNT(*), COUNT(DISTINCT user_id)
		 FROM usage_events WHERE created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY) AND platform <> ''
		 GROUP BY event, platform`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql, day, day, day).Error; err != nil {
			return err
		}
	}
	return nil
}

// ── 看板查询 ──

// UsageOverview 概览卡（近 N 天）
type UsageOverview struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	PV          int64   `json:"pv"`          // 含匿名
	UV          int64   `json:"uv"`          // 仅登录用户（明细表去重，不跨事件相加）
	Events      int64   `json:"events"`      // 事件种类数
	ChatCount   int64   `json:"chatCount"`   // 提问数（来自 chat_messages，非埋点）
	AvgDwellMS  float64 `json:"avgDwellMs"`  // 平均停留（来自 page_leave）
	DwellSample int64   `json:"dwellSample"` // 停留样本数
}

// UsageRankRow 功能排行榜一行
type UsageRankRow struct {
	Event string `json:"event"`
	PV    int64  `json:"pv"`
	UV    int64  `json:"uv"`
}

// UsageSeriesRow 趋势一行
type UsageSeriesRow struct {
	Day        string `json:"day"`
	PV         int64  `json:"pv"`
	UV         int64  `json:"uv"`
	ChatCount  int64  `json:"chatCount"`
}

// UsageFunnelStep 漏斗一步
type UsageFunnelStep struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	PV    int64  `json:"pv"`
}

func dayRange(from, to string) (string, string) {
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	if from == "" {
		from = time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	}
	return from, to
}

// Overview 概览：PV/UV 走明细表（UV 必须去重），提问数走 chat_messages
func (s *UsageService) Overview(from, to string) (*UsageOverview, error) {
	db := database.DB()
	if db == nil {
		return &UsageOverview{}, nil
	}
	from, to = dayRange(from, to)
	out := &UsageOverview{From: from, To: to}

	if err := db.Raw(
		`SELECT COUNT(*), COUNT(DISTINCT user_id), COUNT(DISTINCT event)
		 FROM usage_events WHERE created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)`,
		from, to,
	).Row().Scan(&out.PV, &out.UV, &out.Events); err != nil {
		return nil, err
	}

	// 提问数：服务端本来就看得见，不在埋点里重复采集（§2）
	if err := db.Raw(
		`SELECT COUNT(*) FROM chat_messages WHERE role = 'user' AND created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)`,
		from, to,
	).Row().Scan(&out.ChatCount); err != nil {
		logger.L().Warnw("usage overview chat count failed", "err", err)
	}

	// 平均停留（D2）：明细里取 page_leave 的 dwell_ms；样本为 0 时返回 0
	if err := db.Raw(
		`SELECT COALESCE(AVG(CAST(JSON_EXTRACT(props, '$.dwell_ms') AS UNSIGNED)), 0), COUNT(*)
		 FROM usage_events
		 WHERE event = 'page_leave' AND created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)`,
		from, to,
	).Row().Scan(&out.AvgDwellMS, &out.DwellSample); err != nil {
		logger.L().Warnw("usage overview dwell failed", "err", err)
	}
	return out, nil
}

// EventRanking 功能排行榜：默认读聚合表（快）；dim 传 page/platform 时按该维度下钻
func (s *UsageService) EventRanking(from, to, dim string, limit int) ([]UsageRankRow, error) {
	db := database.DB()
	if db == nil {
		return nil, nil
	}
	from, to = dayRange(from, to)
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	// dim 为空 = 总计维度（dim_key=''）；传 page/platform 则按该维度下钻
	dimKey := ""
	if dim == "page" || dim == "platform" {
		dimKey = dim
	}
	rows := []UsageRankRow{}
	q := db.Table("usage_daily").
		Select("event, SUM(pv) AS pv, SUM(uv) AS uv").
		Where("day >= ? AND day <= ?", from, to).
		Where("dim_key = ?", dimKey)
	if dimKey != "" {
		q = q.Where("dim_value <> ''")
	}
	if err := q.Group("event").Order("pv DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Series 趋势：PV/UV 走明细（UV 要按天去重），提问数走 chat_messages
func (s *UsageService) Series(days int) ([]UsageSeriesRow, error) {
	db := database.DB()
	if db == nil {
		return nil, nil
	}
	if days <= 0 || days > 90 {
		days = 30
	}
	out := []UsageSeriesRow{}

	type aggRow struct {
		Day string
		PV  int64
		UV  int64
	}
	usage := []aggRow{}
	if err := db.Raw(
		`SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS day, COUNT(*) AS pv, COUNT(DISTINCT user_id) AS uv
		 FROM usage_events WHERE created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
		 GROUP BY day ORDER BY day`,
		days-1,
	).Scan(&usage).Error; err != nil {
		return nil, err
	}
	type chatRow struct {
		Day  string
		N    int64
	}
	chats := []chatRow{}
	if err := db.Raw(
		`SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS day, COUNT(*) AS n
		 FROM chat_messages WHERE role = 'user' AND created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
		 GROUP BY day ORDER BY day`,
		days-1,
	).Scan(&chats).Error; err != nil {
		logger.L().Warnw("usage series chat failed", "err", err)
	}

	chatByDay := map[string]int64{}
	for _, c := range chats {
		chatByDay[c.Day] = c.N
	}
	usageByDay := map[string]aggRow{}
	for _, u := range usage {
		usageByDay[u.Day] = u
	}
	// 补齐空缺日期（0 而不是断线，§7 P1 验收要求）
	for i := days - 1; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		u := usageByDay[day]
		out = append(out, UsageSeriesRow{Day: day, PV: u.PV, UV: u.UV, ChatCount: chatByDay[day]})
	}
	return out, nil
}

// Funnel 漏斗：首页 → 对话页 → 首次提问 → 二次提问。
// ⚠️ **PV 口径**（D1 不采集匿名标识）：前三步是次数，最后一步是"同会话 ≥2 问"的会话数。
func (s *UsageService) Funnel(from, to string) ([]UsageFunnelStep, error) {
	db := database.DB()
	if db == nil {
		return nil, nil
	}
	from, to = dayRange(from, to)
	window := "created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)"

	var homeView, chatView, firstAsk, secondAsk int64
	count := func(q string, dst *int64) {
		if err := db.Raw(q, from, to).Row().Scan(dst); err != nil {
			logger.L().Warnw("usage funnel step failed", "err", err)
		}
	}
	count(`SELECT COUNT(*) FROM usage_events WHERE event='page_view' AND page='home' AND `+window, &homeView)
	count(`SELECT COUNT(*) FROM usage_events WHERE event='page_view' AND page='chat' AND `+window, &chatView)
	count(`SELECT COUNT(*) FROM usage_events WHERE event='chat_message_sent' AND `+window, &firstAsk)
	count(`SELECT COUNT(*) FROM (
	         SELECT session_id FROM chat_messages WHERE role='user' AND session_id IS NOT NULL
	           AND created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)
	         GROUP BY session_id HAVING COUNT(*) >= 2
	       ) t`, &secondAsk)

	return []UsageFunnelStep{
		{Key: "home_view", Label: "访问首页", PV: homeView},
		{Key: "chat_view", Label: "进入对话页", PV: chatView},
		{Key: "first_ask", Label: "发出提问", PV: firstAsk},
		{Key: "second_ask", Label: "二次提问（同会话≥2）", PV: secondAsk},
	}, nil
}

// ExportCSV 导出明细（默认最近 7 天；上限 5000 行，避免把看板拖死）
func (s *UsageService) ExportCSV(from, to, event string) (string, error) {
	db := database.DB()
	if db == nil {
		return "", nil
	}
	from, to = dayRange(from, to)

	var rows []model.UsageEvent
	q := db.Where("created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)", from, to)
	if event != "" {
		q = q.Where("event = ?", event)
	}
	if err := q.Order("id DESC").Limit(5000).Find(&rows).Error; err != nil {
		return "", err
	}

	sb := &strings.Builder{}
	sb.WriteString("\ufeff") // UTF-8 BOM：Excel 打开不乱码（与后台既有导出一致）
	w := csv.NewWriter(sb)
	_ = w.Write([]string{"时间", "事件", "用户ID", "会话ID", "页面", "平台", "属性", "客户端时间"})
	for _, r := range rows {
		uid := ""
		if r.UserID != nil {
			uid = strconv.FormatUint(uint64(*r.UserID), 10)
		}
		_ = w.Write([]string{
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.Event, uid, r.SessionID, r.Page, r.Platform, r.Props,
			fmt.Sprintf("%d", r.ClientTS),
		})
	}
	w.Flush()
	return sb.String(), nil
}
