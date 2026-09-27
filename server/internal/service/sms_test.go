package service

// 短信验证码服务单测（手机号注册用，2026-09-27）
//
// 覆盖三类风险（都是"写错了会真花钱或被绕过"的）：
//  1. 手机号归一化/校验（错了要么发不出去、要么把非法号送进通道 ✗）
//  2. 验证码语义：一次性、过期、错误次数上限（错了等于验证形同虚设 ✗）
//  3. 限速：同号间隔 / 同号日上限 / 同 IP 小时上限（错了就是被刷爆短信费 ✗✗）

import (
	"errors"
	"strings"
	"testing"
	"time"

	"teyvat-guide/server/internal/config"
)

// testMockSMS 造一个 mock 通道的短信服务（不发真短信 ✓，验证码可从通道里 peek 出来 ✓）
func testMockSMS() *SMSService {
	return NewSMSService(config.SMSConfig{
		Enabled:             true,
		Provider:            "aliyun",
		Mock:                true,
		CodeTTLMinutes:      5,
		SendIntervalSeconds: 60,
		DailyLimitPerPhone:  10,
		HourlyLimitPerIP:    20,
	})
}

// testClock 可拨动的时钟。
// ⚠️ 必须**同时**注入服务与 mock 通道：只改一个的话，限速用旧钟、验证码过期用新钟，
// 测试会自相矛盾 ✗（第一版就踩了：只改了通道的钟，于是"61 秒后应可重发"仍被限速拦住 ✗）。
type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

// newTestClock 把 s（以及它的 mock 通道）接到同一个可拨动的钟上。
//
// ⚠️ 起点固定为**正午**而不是 time.Now()：日上限用例会往前拨 10 分钟，
// 从真时间起步的话，如果恰好跑在 23:5x，就会跨到第二天 ⇒ `dayCount` 换 key ⇒ 用例偶发失败 ✗
// （2026-09-27 00:0x 实际踩到一次 ✓ 与业务无关，纯粹是测试用真时钟的坑 ✗）。
func newTestClock(s *SMSService) *testClock {
	c := &testClock{t: time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)}
	s.now = c.now
	if m, ok := s.provider.(*mockSMS); ok {
		m.now = c.now
	}
	return c
}

// advance 往前走 d（服务的限速窗口与通道的有效期一起前进 ✓）
func (c *testClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// mockCodeOf 取 mock 通道当前验证码（拿不到即测试失败 —— 说明发送没成功 ✓）
func mockCodeOf(t *testing.T, s *SMSService, phone string) string {
	t.Helper()
	m, ok := s.provider.(*mockSMS)
	if !ok {
		t.Fatal("应为 mock 通道")
	}
	code, ok := m.peek(phone)
	if !ok {
		t.Fatalf("通道里没有 %s 的验证码（发送没成功？）", MaskPhone(phone))
	}
	return code
}

func TestNormalizePhone(t *testing.T) {
	valid := map[string]string{
		"13800138000":     "13800138000",
		" 138 0013 8000 ": "13800138000",
		"+8613800138000":  "13800138000",
		"8613800138000":   "13800138000",
		"138-0013-8000":   "13800138000",
	}
	for in, want := range valid {
		got, ok := NormalizePhone(in)
		if !ok || got != want {
			t.Errorf("NormalizePhone(%q) = (%q,%v)，期望 (%q,true)", in, got, ok, want)
		}
	}
	invalid := []string{
		"", "1380013800", "138001380000", // 位数不对
		"12800138000",  // 第二位不能是 2
		"23800138000",  // 不以 1 开头
		"1380013800a",  // 含字母
		"1990000000",   // 10 位
		"138 0013 800", // 少一位
	}
	for _, in := range invalid {
		if got, ok := NormalizePhone(in); ok {
			t.Errorf("NormalizePhone(%q) 应判非法，实际 (%q,true)", in, got)
		}
	}
}

func TestMaskPhone(t *testing.T) {
	if got := MaskPhone("13812345678"); got != "138****5678" {
		t.Errorf("掩码应为 138****5678，实际 %q", got)
	}
	// 非 11 位原样返回（日志里也别 panic ✗）
	if got := MaskPhone("abc"); got != "abc" {
		t.Errorf("非 11 位应原样返回，实际 %q", got)
	}
}

func TestSMSDisabledByDefault(t *testing.T) {
	s := NewSMSService(config.SMSConfig{}) // enabled=false（默认口径 ✓）
	if s.Enabled() {
		t.Fatal("默认不应启用短信")
	}
	if err := s.SendCode("13800138000", "127.0.0.1"); !errors.Is(err, ErrSMSDisabled) {
		t.Fatalf("未启用时应返回 ErrSMSDisabled，实际 %v", err)
	}
	if err := s.VerifyCode("13800138000", "123456"); !errors.Is(err, ErrSMSDisabled) {
		t.Fatalf("未启用时校验也应拒绝，实际 %v", err)
	}
}

func TestSMSSendAndVerifyOnce(t *testing.T) {
	s := testMockSMS()
	if err := s.SendCode("13800138000", "127.0.0.1"); err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	code := mockCodeOf(t, s, "13800138000")
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		t.Fatalf("验证码应是 6 位数字，实际 %q", code)
	}

	// 错码 → ErrSMSCodeWrong
	if err := s.VerifyCode("13800138000", "000000"); !errors.Is(err, ErrSMSCodeWrong) {
		t.Fatalf("错码应返回 ErrSMSCodeWrong，实际 %v", err)
	}
	// 正确码 → 通过
	if err := s.VerifyCode("13800138000", code); err != nil {
		t.Fatalf("正确码应通过，实际 %v", err)
	}
	// 一次性：同一码再来应失败 ✓（否则一个码能注册多个账号 ✗）
	if err := s.VerifyCode("13800138000", code); !errors.Is(err, ErrSMSCodeWrong) {
		t.Fatalf("验证码应一次性，实际 %v", err)
	}
}

func TestSMSCodeExpires(t *testing.T) {
	s := testMockSMS()
	clock := newTestClock(s)

	if err := s.SendCode("13800138000", ""); err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	code := mockCodeOf(t, s, "13800138000")

	// 5 分钟后（TTL 到点）应失效
	clock.advance(5*time.Minute + time.Second)
	if err := s.VerifyCode("13800138000", code); !errors.Is(err, ErrSMSCodeWrong) {
		t.Fatalf("过期码应失败，实际 %v", err)
	}
}

func TestSMSWrongCodeAttemptsCapped(t *testing.T) {
	s := testMockSMS()
	if err := s.SendCode("13800138000", ""); err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	code := mockCodeOf(t, s, "13800138000")

	// 连错 5 次后作废（防暴力猜 6 位码）
	for i := 0; i < 5; i++ {
		if err := s.VerifyCode("13800138000", "000000"); !errors.Is(err, ErrSMSCodeWrong) {
			t.Fatalf("第 %d 次错码应失败，实际 %v", i+1, err)
		}
	}
	if err := s.VerifyCode("13800138000", "000000"); !errors.Is(err, ErrSMSCodeWrong) {
		t.Fatalf("第 6 次应仍失败，实际 %v", err)
	}
	if err := s.VerifyCode("13800138000", code); !errors.Is(err, ErrSMSCodeWrong) {
		t.Fatalf("超过尝试上限后，正确码也应失效，实际 %v", err)
	}
}

func TestSMSRateLimitInterval(t *testing.T) {
	s := testMockSMS()
	clock := newTestClock(s)

	if err := s.SendCode("13800138000", "127.0.0.1"); err != nil {
		t.Fatalf("首次发送应成功：%v", err)
	}
	// 立刻再发 → 命中 60s 间隔
	err := s.SendCode("13800138000", "127.0.0.1")
	var limited *ErrSMSRateLimited
	if !errors.As(err, &limited) {
		t.Fatalf("立刻重发应被限速，实际 %v", err)
	}
	if limited.Reason != "interval" || limited.RetryAfter <= 0 {
		t.Fatalf("限速原因/等待时间不对：%+v", limited)
	}
	// 另一个号码不受影响（按号限，不是全局）
	if err := s.SendCode("13900139000", "127.0.0.1"); err != nil {
		t.Fatalf("另一个号码不应被限速：%v", err)
	}
	// 61 秒后可重发
	clock.advance(61 * time.Second)
	if err := s.SendCode("13800138000", "127.0.0.1"); err != nil {
		t.Fatalf("间隔过后应可重发：%v", err)
	}
}

func TestSMSRateLimitDailyPerPhone(t *testing.T) {
	s := testMockSMS()
	clock := newTestClock(s)

	for i := 0; i < 10; i++ {
		if err := s.SendCode("13800138000", "127.0.0.1"); err != nil {
			t.Fatalf("第 %d 次应成功：%v", i+1, err)
		}
		clock.advance(61 * time.Second) // 绕开间隔限制，只测日上限
	}
	err := s.SendCode("13800138000", "127.0.0.1")
	var limited *ErrSMSRateLimited
	if !errors.As(err, &limited) || limited.Reason != "daily" {
		t.Fatalf("第 11 次应命中日上限，实际 %v", err)
	}
	if limited.RetryAfter <= 0 || limited.RetryAfter > 24*time.Hour {
		t.Fatalf("日上限的等待时间应在 0~24h 内，实际 %v", limited.RetryAfter)
	}
}

func TestSMSRateLimitHourlyPerIP(t *testing.T) {
	s := testMockSMS()
	clock := newTestClock(s)

	// 同一 IP 用 20 个不同号码（绕开同号限制）把 IP 配额用满
	// 号码 = 13800138 + 两位序号 + 0（共 11 位 ✓，别写成 10 位 ✗）
	for i := 0; i < 20; i++ {
		phone := "13800138" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "0"
		if err := s.SendCode(phone, "10.0.0.1"); err != nil {
			t.Fatalf("第 %d 次（%s）应成功：%v", i+1, phone, err)
		}
	}
	err := s.SendCode("13900139000", "10.0.0.1")
	var limited *ErrSMSRateLimited
	if !errors.As(err, &limited) || limited.Reason != "ip" {
		t.Fatalf("第 21 次应命中 IP 上限，实际 %v", err)
	}
	// 另一个 IP 不受影响 ✓
	if err := s.SendCode("13900139000", "10.0.0.2"); err != nil {
		t.Fatalf("另一个 IP 不应被限速：%v", err)
	}
	// 一小时后恢复
	clock.advance(time.Hour + time.Second)
	if err := s.SendCode("13900139000", "10.0.0.1"); err != nil {
		t.Fatalf("一小时后应恢复：%v", err)
	}
}

// 通道失败时不能占用限速名额 —— 否则"通道抖了一下"就变成"用户 60 秒后才能重试" ✗
func TestSMSRollbackWhenSendFails(t *testing.T) {
	s := NewSMSService(config.SMSConfig{
		Enabled: true, Mock: true, CodeTTLMinutes: 5,
		SendIntervalSeconds: 60, DailyLimitPerPhone: 10, HourlyLimitPerIP: 20,
	})
	failing := &failingProvider{}
	s.provider = failing

	if err := s.SendCode("13800138000", "127.0.0.1"); !errors.Is(err, ErrSMSSendFailed) {
		t.Fatalf("通道失败应返回 ErrSMSSendFailed，实际 %v", err)
	}
	// 换回正常通道后应能立即重试（没被上一次失败占住名额 ✓）
	s.provider = newMockSMS()
	if err := s.SendCode("13800138000", "127.0.0.1"); err != nil {
		t.Fatalf("通道恢复后应能立刻重发：%v", err)
	}
}

type failingProvider struct{}

func (f *failingProvider) Send(string, int) error { return errors.New("channel down") }
func (f *failingProvider) Check(string, string) (bool, error) {
	return false, errors.New("channel down")
}

func TestSMSVerifyCodeInvalidPhone(t *testing.T) {
	s := testMockSMS()
	if err := s.VerifyCode("23800138000", "123456"); !errors.Is(err, ErrPhoneInvalid) {
		t.Fatalf("非法手机号应返回 ErrPhoneInvalid，实际 %v", err)
	}
	if err := s.SendCode("23800138000", "127.0.0.1"); !errors.Is(err, ErrPhoneInvalid) {
		t.Fatalf("非法手机号不应发送，实际 %v", err)
	}
}
