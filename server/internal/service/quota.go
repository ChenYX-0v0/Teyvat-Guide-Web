package service

import (
	"fmt"
	"sync"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
)

// 超限原因（决定 429 的文案）
const (
	QuotaReasonDaily      = "daily"      // 当日额度用完
	QuotaReasonConcurrent = "concurrent" // 同一用户并发过高
	QuotaReasonGlobal     = "global"     // 全站日预算熔断
)

// ErrQuotaExceeded 配额 / 并发超限。Handler 据此返回 429（契约见 docs/04 §2.6）。
type ErrQuotaExceeded struct {
	Reason  string // daily | concurrent | global
	Used    int64  // 已用量：日配额为 token 数，并发为当前在途请求数
	Limit   int64
	ResetAt time.Time // 仅日配额有意义（自然日 0 点重置）
}

func (e *ErrQuotaExceeded) Error() string {
	return fmt.Sprintf("quota exceeded: reason=%s used=%d limit=%d", e.Reason, e.Used, e.Limit)
}

// Message 面向用户的文案（不暴露内部细节；前端直接展示）
func (e *ErrQuotaExceeded) Message() string {
	switch e.Reason {
	case QuotaReasonConcurrent:
		return "派蒙还在回答上一个问题呢，稍等一下再问我好不好？"
	case QuotaReasonGlobal:
		return "今天来问派蒙的人太多啦，额度已经用完，明日 0 点恢复～"
	default:
		return "今日 AI 额度已用完，明日 0 点重置"
	}
}

// QuotaService 用量配额与并发闸门（口径真源：shared-docs/05 §2）。
//
// 为什么放在主服务：AI 服务只认一把 X-AI-Key，**无法区分用户**，所以"按用户限"只能在主服务做。
// 实现是进程内计数：当前单实例部署足够；将来多实例需要换 Redis 计数，届时只替换本文件。
type QuotaService struct {
	cfg  config.QuotaConfig
	repo *repository.ChatRepository

	mu       sync.Mutex
	inflight map[uint]int // userID → 同时进行中的请求数
}

func NewQuotaService(cfg config.QuotaConfig) *QuotaService {
	return &QuotaService{
		cfg:      cfg,
		repo:     repository.NewChatRepository(),
		inflight: make(map[uint]int),
	}
}

// Enabled 是否配置了任何限制
func (s *QuotaService) Enabled() bool {
	return s.cfg.DailyTokensPerUser > 0 || s.cfg.DailyTokensGlobal > 0 || s.cfg.MaxConcurrentPerUser > 0
}

// Acquire 请求前检查额度并占用一个并发名额：
//   - 放行：返回 release（**调用方必须 defer 调用**，用于释放并发占位）
//   - 超限：返回 *ErrQuotaExceeded（此时已占用的名额已回滚）
//
// 必须在**建会话之前**调用；流式请求因此能在任何 SSE 帧写出之前被拒（见 docs/04 §2.6）。
// 额度是**软上限**：不做事前预扣，允许"最后一问"溢出（最坏 = 上限 + 单次最大消耗）。
func (s *QuotaService) Acquire(userID uint) (func(), error) {
	noop := func() {}
	if !s.Enabled() || userID == 0 {
		return noop, nil
	}

	// 并发闸门：先占位再查库（占位失败就不必查库）
	release := func() {}
	if s.cfg.MaxConcurrentPerUser > 0 {
		s.mu.Lock()
		if s.inflight[userID] >= s.cfg.MaxConcurrentPerUser {
			current := s.inflight[userID]
			s.mu.Unlock()
			return noop, &ErrQuotaExceeded{
				Reason: QuotaReasonConcurrent,
				Used:   int64(current),
				Limit:  int64(s.cfg.MaxConcurrentPerUser),
			}
		}
		s.inflight[userID]++
		s.mu.Unlock()
		release = func() {
			s.mu.Lock()
			if s.inflight[userID] > 0 {
				s.inflight[userID]--
			}
			if s.inflight[userID] == 0 {
				delete(s.inflight, userID)
			}
			s.mu.Unlock()
		}
	}

	// 豁免用户：跳过额度检查（仍占用并发名额，避免豁免账号把并发打满）
	if s.isWhitelisted(userID) {
		return release, nil
	}

	since := StartOfToday()
	resetAt := since.AddDate(0, 0, 1)

	if s.cfg.DailyTokensPerUser > 0 {
		used, err := s.repo.SumUserTokensSince(userID, since)
		if err != nil {
			// 统计失败不拦用户：宁可多花一点额度，也不要因为统计故障让所有人都用不了
			logger.L().Warnw("quota: sum user tokens failed, allow request", "user", userID, "err", err)
		} else if used >= int64(s.cfg.DailyTokensPerUser) {
			release()
			return noop, &ErrQuotaExceeded{
				Reason:  QuotaReasonDaily,
				Used:    used,
				Limit:   int64(s.cfg.DailyTokensPerUser),
				ResetAt: resetAt,
			}
		}
	}

	if s.cfg.DailyTokensGlobal > 0 {
		total, err := s.repo.SumTotalTokensSince(since)
		if err != nil {
			logger.L().Warnw("quota: sum total tokens failed, allow request", "err", err)
		} else if total >= int64(s.cfg.DailyTokensGlobal) {
			release()
			return noop, &ErrQuotaExceeded{
				Reason:  QuotaReasonGlobal,
				Used:    total,
				Limit:   int64(s.cfg.DailyTokensGlobal),
				ResetAt: resetAt,
			}
		}
	}

	return release, nil
}

func (s *QuotaService) isWhitelisted(userID uint) bool {
	for _, id := range s.cfg.WhitelistUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// StartOfToday 当天 00:00（**本地时区**）：配额按自然日滚动重置，无需定时任务
func StartOfToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
