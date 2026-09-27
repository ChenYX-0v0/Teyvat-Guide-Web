package service

// L4 分钟级限速单测（2026-09-27 补）
//
// 为什么需要它：日额度按 **token** 计，拦不住"反复刷越界问题"——那些请求几乎不花 token ✗。
// 所以这里要守的是三件事：
//  1. 数的是**次数**，且窗口会滑动（60 秒）；
//  2. **按用户**限，一个人刷不能影响别人 ✓；
//  3. 被限速时**并发占位要还回去**（否则并发名额被永久占住 ✗）+ 豁免用户不受影响 ✓。

import (
	"errors"
	"testing"

	"teyvat-guide/server/internal/config"
)

func TestQuotaRateLimitPerMinute(t *testing.T) {
	setupUsageTestDB(t)
	svc := NewQuotaService(config.QuotaConfig{MaxRequestsPerMinute: 3})
	if !svc.Enabled() {
		t.Fatal("只配了每分钟上限时 Enabled 也应为 true（否则限速会被整体跳过 ✗）")
	}

	// 前 3 次放行
	for i := 1; i <= 3; i++ {
		release, err := svc.Acquire(7)
		if err != nil {
			t.Fatalf("第 %d 次应放行，实际被拒：%v", i, err)
		}
		release()
	}

	// 第 4 次被限速
	_, err := svc.Acquire(7)
	var quotaErr *ErrQuotaExceeded
	if !errors.As(err, &quotaErr) {
		t.Fatalf("第 4 次应被限速，实际 err=%v", err)
	}
	if quotaErr.Reason != QuotaReasonRate {
		t.Fatalf("超限原因应为 rate，实际 %s", quotaErr.Reason)
	}
	if quotaErr.Limit != 3 || quotaErr.Used != 3 {
		t.Fatalf("used/limit 应为 3/3，实际 %d/%d", quotaErr.Used, quotaErr.Limit)
	}
	if quotaErr.ResetAt.IsZero() {
		t.Fatal("限速必须给出可恢复时刻（ResetAt），前端要据此倒计时 ✓")
	}
	if quotaErr.Message() == "" {
		t.Fatal("限速也需要面向用户的文案")
	}

	// ① 别的用户不受影响（按用户限，不是全局限 ✓）
	if release, err := svc.Acquire(8); err != nil {
		t.Fatalf("另一个用户不应被限速：%v", err)
	} else {
		release()
	}
	// ② 被限速时并发占位必须还回去（否则并发名额会被永久占住 ✗）
	svc.mu.Lock()
	left := svc.inflight[7]
	svc.mu.Unlock()
	if left != 0 {
		t.Fatalf("限速拒绝后不应残留并发占位，实际 %d", left)
	}

	// ③ 豁免用户跳过限速（否则管理员自己会被自己的刷测拦死 ✗）
	white := NewQuotaService(config.QuotaConfig{MaxRequestsPerMinute: 1, WhitelistUserIDs: []uint{9}})
	for i := 0; i < 5; i++ {
		release, err := white.Acquire(9)
		if err != nil {
			t.Fatalf("豁免用户第 %d 次不应被限速：%v", i+1, err)
		}
		release()
	}

	// ④ 不配（0）时完全无限速（默认关闭口径 ✓）
	off := NewQuotaService(config.QuotaConfig{})
	for i := 0; i < 50; i++ {
		release, err := off.Acquire(10)
		if err != nil {
			t.Fatalf("未配置限速时不该被拦：%v", err)
		}
		release()
	}
}
