package service

// 短信验证码（阿里云号码认证服务 Dypnsapi）—— 手机号注册的第一道门（2026-09-27 新增）
//
// 三条设计决定：
//  1. **验证码由阿里云生成与校验**（SendSmsVerifyCode + CheckSmsVerifyCode）⇒ 我们**不落库验证码** ✓：
//     少一处短时凭据（泄露面小 ✗），且天然带"有效期 + 一次性"语义 ✓。
//     短信文案里的 `##code##` 由阿里云替换（见 TemplateParam）。
//  2. **我们只做限速**（进程内滑窗，口径同 quota.go：本项目**没有 Redis** ✗）：
//     同号 60s 间隔 / 同号每日上限 / 同 IP 每小时上限 —— 短信按条计费，被刷就是直接烧钱 ✗✗；
//     阿里云侧 `Interval` 参数也会拦一道 ✓（双保险）。
//  3. **mock 模式**：本地开发与单测不真发短信 ✓（自建验证码 + 进程内校验 ✓），验证码只进日志 ✓。
//     ⚠️ 任何模式下都**不把验证码返回给前端** ✗ —— 连 mock 也不返回 ✓（否则"测试习惯"会被带进线上 ✗）。

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dypnsapi "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	aliyuncred "github.com/aliyun/credentials-go/credentials"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/logger"
)

var (
	// ErrPhoneInvalid 手机号格式不对（大陆 11 位，1[3-9] 开头）
	ErrPhoneInvalid = errors.New("手机号格式不正确")
	// ErrSMSDisabled 短信服务未开启/未配置好（Handler 映射 503，文案不暴露内部细节）
	ErrSMSDisabled = errors.New("短信服务暂未开启，请联系管理员")
	// ErrSMSCodeWrong 验证码错误或已过期（阿里云侧一次性语义 ⇒ 用过的码再来也是这条 ✓）
	ErrSMSCodeWrong = errors.New("验证码错误或已过期")
	// ErrSMSSendFailed 通道侧失败（签名/模板/余额/AK 权限等，细节只进日志）
	ErrSMSSendFailed = errors.New("验证码发送失败，请稍后重试")
)

// ErrSMSRateLimited 命中限速（同号间隔 / 同号日上限 / 同 IP 小时上限）。
// Handler 映射 429，并把 RetryAfter 作为 `retryAfterSeconds` 下发 ✓（前端据此倒计时）。
type ErrSMSRateLimited struct {
	Reason     string // interval | daily | ip（只进日志）
	RetryAfter time.Duration
}

func (e *ErrSMSRateLimited) Error() string {
	sec := int(e.RetryAfter.Seconds())
	if sec < 1 {
		sec = 1
	}
	if sec >= 60 && sec%60 == 0 {
		return fmt.Sprintf("发送太频繁了，请 %d 分钟后再试", sec/60)
	}
	return fmt.Sprintf("发送太频繁了，请 %d 秒后再试", sec)
}

// 大陆手机号：1 开头，第二位 3~9，共 11 位。**只收大陆号**（阿里云该接口也只支持大陆 ✓）
var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// NormalizePhone 归一化手机号：去掉空格/连字符与 `+86` / `86` 前缀；返回 (号码, 是否合法)
func NormalizePhone(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	s = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(s)
	s = strings.TrimPrefix(s, "+")
	if strings.HasPrefix(s, "86") && len(s) == 13 {
		s = s[2:]
	}
	if !phonePattern.MatchString(s) {
		return "", false
	}
	return s, true
}

// MaskPhone 手机号脱敏（日志/默认昵称用）：13812345678 → 138****5678
func MaskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}

// smsProvider 通道抽象：真实阿里云 / mock 各自实现，限速逻辑与服务商无关 ✓
type smsProvider interface {
	// Send 发送验证码（由通道侧生成并保存，ttlMinutes 为有效期）
	Send(phone string, ttlMinutes int) error
	// Check 校验验证码（成功即作废，一次性）
	Check(phone, code string) (bool, error)
}

// SMSService 短信验证码服务：**限速 + 通道调用**（验证码本身不落库 ✓）
type SMSService struct {
	cfg      config.SMSConfig
	provider smsProvider

	mu       sync.Mutex
	lastSent map[string]time.Time   // 手机号 → 上次发送时刻（间隔限制）
	dayCount map[string]int         // 手机号@日期 → 当日发送次数
	ipHits   map[string][]time.Time // IP → 最近一小时发送时刻

	// now 可注入（单测把时间往前拨，验证间隔/日限/IP 限 ✓）
	now func() time.Time
}

// NewSMSService 构造：未开启 / 未配置好时 provider 为 nil —— 所有发送都会返回 ErrSMSDisabled ✓
// （**启动不做任何网络调用** ✗：凭据链与端点都在首次发送时才真正用到 ✓）
func NewSMSService(cfg config.SMSConfig) *SMSService {
	s := &SMSService{
		cfg:      cfg,
		lastSent: make(map[string]time.Time),
		dayCount: make(map[string]int),
		ipHits:   make(map[string][]time.Time),
		now:      time.Now,
	}
	if !cfg.Enabled {
		logger.L().Infow("sms disabled", "hint", "TEYVAT_SMS_ENABLED=false 时手机号注册不可用")
		return s
	}
	if cfg.Mock {
		s.provider = newMockSMS()
		logger.L().Warnw("sms mock enabled", "hint", "不真发短信；验证码只打日志（仅限本地/联调 ✗ 别开在线上）")
		return s
	}
	p, err := newAliyunSMS(cfg)
	if err != nil {
		// 只记日志不 panic：短信不可用时其余功能照常（注册接口会明确拒绝 ✓）
		logger.L().Errorw("sms provider init failed", "err", err)
		return s
	}
	s.provider = p
	logger.L().Infow("sms ready", "provider", cfg.Provider, "sign", cfg.SignName, "template", cfg.TemplateCode)
	return s
}

// Enabled 短信通道是否可用（配置开关 + provider 就绪）
func (s *SMSService) Enabled() bool { return s != nil && s.provider != nil }

// TTLSeconds 验证码有效期（秒），供前端展示
func (s *SMSService) TTLSeconds() int { return s.cfg.CodeTTLMinutes * 60 }

// SendIntervalSeconds 同号重发间隔（秒），供前端倒计时
func (s *SMSService) SendIntervalSeconds() int { return s.cfg.SendIntervalSeconds }

// Mock 是否处于 mock 模式（/api/auth/config 会下发，便于前端提示"验证码见服务端日志"）
func (s *SMSService) Mock() bool { return s != nil && s.cfg.Mock }

// SendCode 发送验证码：先过三重限速，再调通道。
// clientIP 只用于限速（**不落库、不打日志原文** ✓），可为空字符串。
func (s *SMSService) SendCode(phone, clientIP string) error {
	if !s.Enabled() {
		return ErrSMSDisabled
	}
	normalized, ok := NormalizePhone(phone)
	if !ok {
		return ErrPhoneInvalid
	}
	if err := s.allow(normalized, clientIP); err != nil {
		return err
	}
	if err := s.provider.Send(normalized, s.cfg.CodeTTLMinutes); err != nil {
		// 通道失败时**不占用**限速名额（用户可立即重试；否则要等 60s 才能重发 ✗）
		s.rollback(normalized, clientIP)
		logger.L().Errorw("sms send failed", "phone", MaskPhone(normalized), "err", err)
		return ErrSMSSendFailed
	}
	s.commit(normalized, clientIP) // ⚠️ 必须记：不记则限速形同虚设 ✗
	logger.L().Infow("sms sent", "phone", MaskPhone(normalized), "ttlMin", s.cfg.CodeTTLMinutes)
	return nil
}

// VerifyCode 校验验证码；失败返回 ErrSMSCodeWrong（不区分"错"与"过期"—— 不给出试探信号 ✓）
func (s *SMSService) VerifyCode(phone, code string) error {
	if !s.Enabled() {
		return ErrSMSDisabled
	}
	normalized, ok := NormalizePhone(phone)
	if !ok {
		return ErrPhoneInvalid
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrSMSCodeWrong
	}
	pass, err := s.provider.Check(normalized, code)
	if err != nil {
		logger.L().Errorw("sms check failed", "phone", MaskPhone(normalized), "err", err)
		return ErrSMSSendFailed
	}
	if !pass {
		return ErrSMSCodeWrong
	}
	return nil
}

// allow 三重限速（全部命中才放行）；命中时返回 *ErrSMSRateLimited
func (s *SMSService) allow(phone, ip string) error {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()

	// ① 同号间隔（默认 60s）：防连点
	if s.cfg.SendIntervalSeconds > 0 {
		if last, ok := s.lastSent[phone]; ok {
			if wait := time.Duration(s.cfg.SendIntervalSeconds)*time.Second - now.Sub(last); wait > 0 {
				return &ErrSMSRateLimited{Reason: "interval", RetryAfter: wait}
			}
		}
	}
	// ② 同号每日上限：防单号被刷
	if s.cfg.DailyLimitPerPhone > 0 && s.dayCount[phone+"@"+now.Format("20060102")] >= s.cfg.DailyLimitPerPhone {
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(24 * time.Hour)
		return &ErrSMSRateLimited{Reason: "daily", RetryAfter: next.Sub(now)}
	}
	// ③ 同 IP 每小时上限：防换号刷（IP 只存在这张进程内表里 ✓）
	if s.cfg.HourlyLimitPerIP > 0 && ip != "" {
		cut := now.Add(-time.Hour)
		kept := keepAfter(s.ipHits[ip], cut)
		if len(kept) >= s.cfg.HourlyLimitPerIP {
			return &ErrSMSRateLimited{Reason: "ip", RetryAfter: kept[0].Add(time.Hour).Sub(now)}
		}
	}
	return nil
}

// commit 记录一次成功发送（由 SendCode 在通道成功后调用）
func (s *SMSService) commit(phone, ip string) {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSent[phone] = now
	s.dayCount[phone+"@"+now.Format("20060102")]++

	// 顺手清理过期计数（进程长跑不涨内存 ✓）
	cut := now.Add(-time.Hour)
	if ip != "" {
		// ⚠️ 必须把本次也 append 进去：只裁剪不追加 ⇒ 同 IP 限速永远不生效 ✗
		s.ipHits[ip] = append(keepAfter(s.ipHits[ip], cut), now)
	}
	day := now.Format("20060102")
	for k := range s.dayCount {
		if !strings.HasSuffix(k, "@"+day) {
			delete(s.dayCount, k)
		}
	}
}

// rollback 通道失败时把已记的名额撤掉（让用户能立刻重试 ✓）
func (s *SMSService) rollback(phone, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.lastSent, phone)
}

// keepAfter 返回 time 切片中晚于 cut 的部分（滑动窗口）
func keepAfter(ts []time.Time, cut time.Time) []time.Time {
	kept := make([]time.Time, 0, len(ts))
	for _, t := range ts {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	return kept
}

// ------------------------------ 真实通道：阿里云 Dypnsapi ------------------------------

type aliyunSMS struct {
	cfg    config.SMSConfig
	client *dypnsapi.Client
}

// newAliyunSMS 建客户端。**优先显式 AK**；两者都空则走"无 AK"凭据链 ✓
// （ECS 实例 RAM 角色 / 环境变量 ALIBABA_CLOUD_ACCESS_KEY_ID 等，安全且不用把密钥写进环境 ✓）
func newAliyunSMS(cfg config.SMSConfig) (*aliyunSMS, error) {
	var cred aliyuncred.Credential
	if cfg.AccessKeyID != "" && cfg.AccessKeySecret != "" {
		c, err := aliyuncred.NewCredential(&aliyuncred.Config{
			AccessKeyId:     tea.String(cfg.AccessKeyID),
			AccessKeySecret: tea.String(cfg.AccessKeySecret),
			Type:            tea.String("access_key"),
		})
		if err != nil {
			return nil, fmt.Errorf("初始化显式 AK 失败：%w", err)
		}
		cred = c
	} else {
		c, err := aliyuncred.NewCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("初始化无 AK 凭据链失败（可改配 TEYVAT_SMS_ACCESS_KEY_ID/SECRET）：%w", err)
		}
		cred = c
	}
	openCfg := &openapi.Config{Credential: cred}
	// Endpoint 见 https://api.aliyun.com/product/Dypnsapi（默认 dypnsapi.aliyuncs.com）
	openCfg.Endpoint = tea.String(cfg.Endpoint)
	client, err := dypnsapi.NewClient(openCfg)
	if err != nil {
		return nil, fmt.Errorf("初始化 Dypnsapi 客户端失败：%w", err)
	}
	return &aliyunSMS{cfg: cfg, client: client}, nil
}

// Send 阿里云生成并保存验证码（我们拿不到明文 ✓），再按模板下发短信
func (a *aliyunSMS) Send(phone string, ttlMinutes int) error {
	req := &dypnsapi.SendSmsVerifyCodeRequest{
		PhoneNumber:  tea.String(phone),
		SignName:     tea.String(a.cfg.SignName),
		TemplateCode: tea.String(a.cfg.TemplateCode),
		// 模板占位：##code## = 阿里云生成的验证码；min = 文案里写的有效分钟数（与 ValidTime 保持一致 ✓）
		TemplateParam: tea.String(fmt.Sprintf(`{"code":"##code##","min":"%d"}`, ttlMinutes)),
		CodeLength:    tea.Int64(6), // 6 位数字
		CodeType:      tea.Int64(1), // 1 = 纯数字
		ValidTime:     tea.Int64(int64(ttlMinutes) * 60),
		Interval:      tea.Int64(int64(a.cfg.SendIntervalSeconds)),
		// 1 = 新码覆盖旧码（避免"同时存在多个有效码"，也让重复发送不浪费额度 ✓）
		DuplicatePolicy: tea.Int64(1),
		// ⚠️ 永不要求返回验证码明文 ✗（返回了就会进日志/响应体，等于把码送给攻击者）
		ReturnVerifyCode: tea.Bool(false),
	}
	resp, err := a.client.SendSmsVerifyCodeWithOptions(req, &util.RuntimeOptions{})
	if err != nil {
		return fmt.Errorf("SendSmsVerifyCode 调用失败：%w", err)
	}
	if resp == nil || resp.Body == nil {
		return errors.New("SendSmsVerifyCode 返回空响应")
	}
	if !tea.BoolValue(resp.Body.Success) || tea.StringValue(resp.Body.Code) != "OK" {
		return fmt.Errorf("SendSmsVerifyCode 业务失败：code=%s message=%s",
			tea.StringValue(resp.Body.Code), tea.StringValue(resp.Body.Message))
	}
	return nil
}

// Check 校验验证码（阿里云侧一次性：校验通过即失效 ✓）
func (a *aliyunSMS) Check(phone, code string) (bool, error) {
	resp, err := a.client.CheckSmsVerifyCodeWithOptions(&dypnsapi.CheckSmsVerifyCodeRequest{
		PhoneNumber:    tea.String(phone),
		VerifyCode:     tea.String(code),
		CaseAuthPolicy: tea.Int64(1), // 不区分大小写（数字码无影响，留着稳妥 ✓）
	}, &util.RuntimeOptions{})
	if err != nil {
		return false, fmt.Errorf("CheckSmsVerifyCode 调用失败：%w", err)
	}
	if resp == nil || resp.Body == nil {
		return false, errors.New("CheckSmsVerifyCode 返回空响应")
	}
	if tea.StringValue(resp.Body.Code) != "OK" {
		// 业务码非 OK = 校验没通过（不是系统错误 ⇒ 返回 false 让上层回"验证码错误" ✓）
		logger.L().Warnw("sms verify rejected",
			"phone", MaskPhone(phone), "code", tea.StringValue(resp.Body.Code),
			"message", tea.StringValue(resp.Body.Message))
		return false, nil
	}
	if resp.Body.Model == nil {
		return false, nil
	}
	return tea.StringValue(resp.Body.Model.VerifyResult) == "PASS", nil
}

// ------------------------------ mock 通道（本地/联调/单测） ------------------------------

type mockCode struct {
	code      string
	expiresAt time.Time
	attempts  int
}

type mockSMS struct {
	mu    sync.Mutex
	codes map[string]mockCode
	now   func() time.Time
}

func newMockSMS() *mockSMS {
	return &mockSMS{codes: make(map[string]mockCode), now: time.Now}
}

// Send mock：自建 6 位验证码并打日志（⚠️ 只打日志，**绝不返回给调用方** ✗）
func (m *mockSMS) Send(phone string, ttlMinutes int) error {
	code, err := randomDigits(6)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.codes[phone] = mockCode{code: code, expiresAt: m.now().Add(time.Duration(ttlMinutes) * time.Minute)}
	m.mu.Unlock()
	// 只进日志：本地开发时从 `docker logs` / 终端里看 ✓
	logger.L().Warnw("sms mock code (仅本地/联调)", "phone", MaskPhone(phone), "code", code,
		"ttlMin", ttlMinutes, "hint", "mock 模式不真发短信 ✓")
	return nil
}

// Check mock：比对 + 过期 + 一次性 + 最多 5 次尝试（贴近阿里云的行为 ✓）
func (m *mockSMS) Check(phone, code string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.codes[phone]
	if !ok {
		return false, nil
	}
	if m.now().After(entry.expiresAt) {
		delete(m.codes, phone)
		return false, nil
	}
	entry.attempts++
	if entry.attempts > 5 {
		delete(m.codes, phone)
		return false, nil
	}
	if entry.code != code {
		m.codes[phone] = entry
		return false, nil
	}
	delete(m.codes, phone) // 一次性 ✓
	return true, nil
}

// peek **仅供单测**读取当前验证码：mock 通道自建的码需要能被测试拿到，
// 否则就没法测"注册链路"✓。⚠️ 生产路径绝不调用它 ✗ —— 真实通道（阿里云）的码我们
// 本来就拿不到（`ReturnVerifyCode=false` ✓），这里也没有对外的暴露入口 ✓。
func (m *mockSMS) peek(phone string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.codes[phone]
	return entry.code, ok
}

// randomDigits 生成 n 位纯数字验证码（crypto/rand，别用 math/rand ✗）
func randomDigits(n int) (string, error) {
	var b strings.Builder
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + v.Int64()))
	}
	return b.String(), nil
}
