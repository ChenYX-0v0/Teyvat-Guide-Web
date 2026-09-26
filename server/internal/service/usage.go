package service

// 埋点采集服务（真源：shared-docs/07-前端埋点与使用统计.md §4.3）
//
// 三条不可违背的原则（改这里之前先读一遍）：
//  1. **永不阻塞业务**：入队即返回，落库交给后台 worker 批量做；
//  2. **永不给调用方报错**：未知事件 / props 超限 / 命中限流 —— 一律静默丢弃并计数
//     （不返回错误，免得被拿来探测"什么事件名有效"）；
//  3. **永不落身份标识**：D1 定案不采集匿名标识；IP 只存在于进程内限流表，**不入库**。
//
// 与既有能力的分工（§2）：提问数 / token 成本 / 无关问题占比等"服务端本来就看得见"的指标
// 继续从 chat_messages 等表算，**不要在这里重复采集**。

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/pkg/logger"
)

// usageEventProps 事件白名单：事件名 → 允许保留的 props 键（§5 事件字典 v1）。
// 不在此表的事件名一律丢弃；不在此名单的 props 键一律剥掉（防脏数据）。
var usageEventProps = map[string][]string{
	"page_view":            {"referrer_page", "is_logged_in"},
	"page_leave":           {"dwell_ms"}, // D2：停留时长（上下截断见 normalize）
	"home_cta_click":       {"target"},
	"chat_message_sent":    {"source", "has_uid", "is_first_in_session"},
	"chat_stream_failed":   {"reason", "elapsed_ms"},
	"chat_new_session":     {},
	"chat_session_opened":  {"index"},
	"chat_session_deleted": {},
	"chat_preset_expanded": {"index"},
	"account_uid_query":    {"result", "force_refresh"},
	"account_uid_bind":     {},
	"account_unbind":       {},
	"auth_login":           {"result"},
	"web_error":            {"message_hash"}, // 只存哈希，不存原文
}

// 页面 / 平台取值白名单（不在这里的一律置空，避免自由文本把维度打散）
var (
	usagePages      = map[string]bool{"home": true, "chat": true, "account": true, "wiki": true, "login": true}
	usagePlatforms  = map[string]bool{"mobile": true, "desktop": true}
	usageMaxDwellMS = int64(30 * 60 * 1000) // >30min 记为超长（挂机不算停留）
)

type usageCounter struct {
	at time.Time
	n  int
}

// UsageService 埋点采集（进程内队列 + 单 worker 批量落库）
type UsageService struct {
	cfg config.UsageConfig
	ch  chan []model.UsageEvent

	mu     sync.Mutex
	byIP   map[string]*usageCounter
	byUser map[uint]*usageCounter

	droppedUnknown atomic.Int64 // 事件名不在白名单
	droppedInvalid atomic.Int64 // props 超限 / 停留时长过短（误触）
	droppedLimited atomic.Int64 // 命中限流或队列满
}

func NewUsageService(cfg config.UsageConfig) *UsageService {
	s := &UsageService{
		cfg:    cfg,
		ch:     make(chan []model.UsageEvent, 256),
		byIP:   make(map[string]*usageCounter),
		byUser: make(map[uint]*usageCounter),
	}
	if cfg.Enabled {
		go s.worker()
		go s.cleanupLoop()
		go s.statsLoop()
		logger.L().Infow("usage ingest enabled",
			"retention_days", cfg.RetentionDays, "max_batch", cfg.MaxBatch)
	} else {
		logger.L().Infow("usage ingest disabled (usage.enabled=false)")
	}
	return s
}

// Ingest 接收一批事件，返回真正入队的条数。**调用方不需要处理失败**（原则 2）。
func (s *UsageService) Ingest(userID *uint, ip string, req dto.UsageIngestRequest) int {
	if !s.cfg.Enabled {
		return 0
	}
	if !s.allow(ip, userID) {
		s.droppedLimited.Add(int64(len(req.Events)))
		return 0
	}

	out := make([]model.UsageEvent, 0, len(req.Events))
	for _, in := range req.Events {
		ev, ok := s.normalize(userID, in)
		if !ok {
			continue // normalize 内部已计数
		}
		out = append(out, ev)
	}
	if len(out) == 0 {
		return 0
	}

	select {
	case s.ch <- out: // 入队即返回；worker 负责落库
		return len(out)
	default:
		// 队列满 = 落库跟不上：丢弃，而不是阻塞请求（原则 1）
		s.droppedLimited.Add(int64(len(out)))
		return 0
	}
}

// normalize 校验并收敛单条事件；false = 丢弃（已计数）
func (s *UsageService) normalize(userID *uint, in dto.UsageEventInput) (model.UsageEvent, bool) {
	allowed, known := usageEventProps[in.Event]
	if !known {
		s.droppedUnknown.Add(1)
		return model.UsageEvent{}, false
	}

	props := make(map[string]any, len(allowed))
	if in.Props != nil {
		for _, k := range allowed {
			v, ok := in.Props[k]
			if !ok || v == nil {
				continue
			}
			props[k] = v
		}
	}

	// D2：停留时长上下截断 —— <1s 视为误触丢弃，>30min 记为超长
	if in.Event == "page_leave" {
		ms, ok := toInt64(props["dwell_ms"])
		if !ok {
			s.droppedInvalid.Add(1)
			return model.UsageEvent{}, false
		}
		if ms < 1000 {
			s.droppedInvalid.Add(1)
			return model.UsageEvent{}, false
		}
		if ms > usageMaxDwellMS {
			ms = usageMaxDwellMS
		}
		props["dwell_ms"] = ms
	}

	raw, err := json.Marshal(props)
	if err != nil {
		s.droppedInvalid.Add(1)
		return model.UsageEvent{}, false
	}
	if s.cfg.MaxPropsBytes > 0 && len(raw) > s.cfg.MaxPropsBytes {
		s.droppedInvalid.Add(1)
		return model.UsageEvent{}, false
	}

	page, platform := "", ""
	if usagePages[in.Page] {
		page = in.Page
	}
	if usagePlatforms[in.Platform] {
		platform = in.Platform
	}

	return model.UsageEvent{
		Event:        in.Event,
		EventVersion: 1,
		UserID:       userID,
		SessionID:    truncateStr(in.SessionID, 64),
		Page:         page,
		Platform:     platform,
		Props:        string(raw),
		ClientTS:     in.ClientTS,
		CreatedAt:    time.Now(), // 统计以服务端时间为准
	}, true
}

// allow 固定 1 分钟窗口限流（每 IP / 每登录用户各一个）。超限返回 false，静默丢弃。
func (s *UsageService) allow(ip string, userID *uint) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if ip != "" && s.cfg.IPPerMinute > 0 {
		c := s.byIP[ip]
		if c == nil || now.Sub(c.at) >= time.Minute {
			c = &usageCounter{at: now}
			s.byIP[ip] = c
		}
		if c.n >= s.cfg.IPPerMinute {
			return false
		}
		c.n++
	}
	if userID != nil && s.cfg.UserPerMinute > 0 {
		c := s.byUser[*userID]
		if c == nil || now.Sub(c.at) >= time.Minute {
			c = &usageCounter{at: now}
			s.byUser[*userID] = c
		}
		if c.n >= s.cfg.UserPerMinute {
			return false
		}
		c.n++
	}
	// 顺带清理过期窗口，避免 map 无限增长（IP 只在这里存在，永不落库）
	if len(s.byIP) > 4096 {
		for k, v := range s.byIP {
			if now.Sub(v.at) >= time.Minute {
				delete(s.byIP, k)
			}
		}
	}
	return true
}

// worker 单协程批量落库：写失败只记日志（埋点不能影响业务）
func (s *UsageService) worker() {
	for batch := range s.ch {
		db := database.DB()
		if db == nil {
			continue
		}
		if err := db.Create(&batch).Error; err != nil {
			logger.L().Warnw("usage events insert failed", "count", len(batch), "err", err)
		}
	}
}

// cleanupLoop 明细保留期清理（默认 90 天，`usage.retention_days`；0 = 不清理）
func (s *UsageService) cleanupLoop() {
	if s.cfg.RetentionDays <= 0 {
		return
	}
	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	for range tick.C {
		db := database.DB()
		if db == nil {
			continue
		}
		cut := time.Now().AddDate(0, 0, -s.cfg.RetentionDays)
		res := db.Where("created_at < ?", cut).Delete(&model.UsageEvent{})
		if res.Error != nil {
			logger.L().Warnw("usage events cleanup failed", "err", res.Error)
			continue
		}
		if res.RowsAffected > 0 {
			logger.L().Infow("usage events cleaned", "deleted", res.RowsAffected, "before", cut.Format(time.RFC3339))
		}
	}
}

// statsLoop 丢弃计数打点：每 5 分钟把非零计数打一行日志（P0 不引指标系统，先靠日志）
func (s *UsageService) statsLoop() {
	tick := time.NewTicker(5 * time.Minute)
	defer tick.Stop()
	for range tick.C {
		unknown := s.droppedUnknown.Swap(0)
		invalid := s.droppedInvalid.Swap(0)
		limited := s.droppedLimited.Swap(0)
		if unknown+invalid+limited > 0 {
			logger.L().Warnw("usage events dropped",
				"unknown_event", unknown, "invalid", invalid, "limited", limited)
		}
	}
}

// toInt64 兼容 json 解出来的 float64 与直接传入的整数
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case uint:
		return int64(n), true
	default:
		return 0, false
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
