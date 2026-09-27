package service

// 邀请码单测（2026-09-28）—— 守的是用户定的四条口径：
//  1. 邀请码在表单里**手机号上方**（前端 ✓，这里测服务端）；
//  2. **不填不能发短信** ✓、**码不对也不能发短信** ✓（= 错的码一条都不发 ✗）；
//  3. **一次性**：注册成功即作废 ✓；
//  4. 反过来的边界也很重要：**注册失败（短信码错 / 手机号已存在）不能把码烧掉** ✗✓
//     —— 否则用户填错一次码就白拿了一张入场券 ✗。

import (
	"errors"
	"strings"
	"testing"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/repository"
)

// testInviteAuthConfig 要求邀请码的配置（其余同 testAuthConfig ✓）
func testInviteAuthConfig(allowRegister bool) config.AuthConfig {
	c := testAuthConfig(allowRegister)
	c.RequireInviteCode = true
	return c
}

// seedInvite 直接落一张可用的邀请码
func seedInvite(t *testing.T, code string) {
	t.Helper()
	if err := repository.NewInviteRepository().Create(&model.InviteCode{Code: code, Enabled: true}); err != nil {
		t.Fatalf("落邀请码失败：%v", err)
	}
}

// assertNoCodeSent 断言"通道里没有码"= 确实没发短信 ✓（mock 通道自建码 ✓）
func assertNoCodeSent(t *testing.T, s *SMSService, phone string) {
	t.Helper()
	if m, ok := s.provider.(*mockSMS); ok {
		if _, exists := m.peek(phone); exists {
			t.Fatal("不该发码，但通道里已经有码了 ✗")
		}
	}
}

func TestNormalizeInviteCode(t *testing.T) {
	cases := map[string]string{
		"abcd2345":    "ABCD2345",
		" abcd-2345 ": "ABCD2345",
		"ABCD 2345":   "ABCD2345",
		"a_b-c—d2345": "ABCD2345",
		"":            "",
		"   ":         "",
	}
	for in, want := range cases {
		if got := NormalizeInviteCode(in); got != want {
			t.Errorf("NormalizeInviteCode(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestInviteCheckStates(t *testing.T) {
	setupUsageTestDB(t)
	svc := NewInviteService()
	code := "ABCD2345"
	seedInvite(t, code)

	// 空 → ErrInviteRequired（前端"不填不能发短信"的服务端对应 ✓）
	if err := svc.Check(""); !errors.Is(err, ErrInviteRequired) {
		t.Fatalf("空码应返回 ErrInviteRequired，实际 %v", err)
	}
	// 格式不对 → 直接 ErrInviteInvalid（省一次查询 ✓）
	if err := svc.Check("abc"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("格式不对应返回 ErrInviteInvalid，实际 %v", err)
	}
	// 不存在 → ErrInviteInvalid
	if err := svc.Check("ZZZZ9999"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("不存在的码应返回 ErrInviteInvalid，实际 %v", err)
	}
	// 大小写/连字符容错 → 通过 ✓（人工转达最容易抄错这两处 ✓）
	if err := svc.Check(" abcd-2345 "); err != nil {
		t.Fatalf("容错后应通过，实际 %v", err)
	}
	// 停用 → ErrInviteInvalid
	if err := svc.SetEnabled(code, false); err != nil {
		t.Fatalf("停用失败：%v", err)
	}
	if err := svc.Check(code); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("停用后应返回 ErrInviteInvalid，实际 %v", err)
	}
	// 再启用 → 通过 ✓（停用是可逆的，不是删除 ✓）
	if err := svc.SetEnabled(code, true); err != nil {
		t.Fatalf("重新启用失败：%v", err)
	}
	if err := svc.Check(code); err != nil {
		t.Fatalf("重新启用后应通过，实际 %v", err)
	}
	// 停用不存在的码 → ErrInviteInvalid（404 语义 ✓）
	if err := svc.SetEnabled("QQQQ7777", false); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("停用不存在的码应返回 ErrInviteInvalid，实际 %v", err)
	}
}

// 口径 2：不填 / 不对 ⇒ **一条短信都不发** ✓
func TestSendSMSCodeRequiresUsableInvite(t *testing.T) {
	setupUsageTestDB(t)
	seedInvite(t, "ABCD2345")
	sms := testMockSMS()
	svc := NewAuthService(testInviteAuthConfig(true), sms, database.DB())

	// 不填
	if err := svc.SendSMSCode(testPhone, "127.0.0.1", ""); !errors.Is(err, ErrInviteRequired) {
		t.Fatalf("不填邀请码应返回 ErrInviteRequired，实际 %v", err)
	}
	assertNoCodeSent(t, sms, testPhone)

	// 填错
	if err := svc.SendSMSCode(testPhone, "127.0.0.1", "ZZZZ9999"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("错码应返回 ErrInviteInvalid，实际 %v", err)
	}
	assertNoCodeSent(t, sms, testPhone)

	// 填对 → 才真的发 ✓
	if err := svc.SendSMSCode(testPhone, "127.0.0.1", "abcd-2345"); err != nil {
		t.Fatalf("正确邀请码应放行：%v", err)
	}
	if _, ok := sms.provider.(*mockSMS).peek(testPhone); !ok {
		t.Fatal("正确邀请码应真的发出验证码 ✗")
	}

	// 开关关掉时：不需要邀请码（既有行为不变 ✓）
	noNeed := NewAuthService(testAuthConfig(true), testMockSMS(), database.DB())
	if err := noNeed.SendSMSCode("13900139000", "127.0.0.1", ""); err != nil {
		t.Fatalf("require_invite_code=false 时不该要求邀请码：%v", err)
	}
}

// 口径 3：注册成功 ⇒ 码作废；同码不能再用 ✓
func TestRegisterConsumesInviteOnce(t *testing.T) {
	setupUsageTestDB(t)
	seedInvite(t, "ABCD2345")
	sms := testMockSMS()
	svc := NewAuthService(testInviteAuthConfig(true), sms, database.DB())

	if err := sms.SendCode(testPhone, "127.0.0.1"); err != nil {
		t.Fatalf("发码失败：%v", err)
	}
	code := mockCodeOf(t, sms, testPhone)
	u, err := svc.Register(testPhone, code, testPass, "", "ABCD2345")
	if err != nil {
		t.Fatalf("注册应成功：%v", err)
	}

	// 码上留痕：已用 + 用了谁 + 哪个手机号 ✓
	got, err := repository.NewInviteRepository().Get("ABCD2345")
	if err != nil || got == nil {
		t.Fatalf("应能查到该码：%v", err)
	}
	if !got.Used() || got.UsedBy == nil || *got.UsedBy != u.ID || got.UsedPhone != testPhone {
		t.Fatalf("占用留痕不对：%+v", got)
	}

	// 同一张码再注册（换个号）→ ErrInviteUsed ✓，且**第二个号也没被建出来** ✓
	sms2 := testMockSMS()
	svc2 := NewAuthService(testInviteAuthConfig(true), sms2, database.DB())
	if err := sms2.SendCode("13900139000", "127.0.0.1"); err != nil {
		t.Fatalf("发码失败：%v", err)
	}
	code2 := mockCodeOf(t, sms2, "13900139000")
	if _, err := svc2.Register("13900139000", code2, testPass, "", "ABCD2345"); !errors.Is(err, ErrInviteUsed) {
		t.Fatalf("已用的码应返回 ErrInviteUsed，实际 %v", err)
	}
	if u2, _ := svc2.repo.FindByPhone("13900139000"); u2 != nil {
		t.Fatal("码被占用时应回滚，不该建出第二个账号 ✗")
	}
}

// 口径 4（边界）：注册**失败**时不能把码烧掉 ✓✓
func TestRegisterFailuresKeepInviteUsable(t *testing.T) {
	setupUsageTestDB(t)
	seedInvite(t, "ABCD2345")
	sms := testMockSMS()
	svc := NewAuthService(testInviteAuthConfig(true), sms, database.DB())

	// ① 短信验证码填错 ⇒ 报错，但码仍可用 ✓
	if err := sms.SendCode(testPhone, "127.0.0.1"); err != nil {
		t.Fatalf("发码失败：%v", err)
	}
	if _, err := svc.Register(testPhone, "000000", testPass, "", "ABCD2345"); !errors.Is(err, ErrSMSCodeWrong) {
		t.Fatalf("错短信码应返回 ErrSMSCodeWrong，实际 %v", err)
	}
	if err := NewInviteService().Check("ABCD2345"); err != nil {
		t.Fatalf("短信码错时不该消耗邀请码，实际 %v", err)
	}

	// ② 手机号已注册 ⇒ 报错，码仍可用 ✓
	if err := sms.SendCode("13900139000", "127.0.0.1"); err != nil {
		t.Fatalf("发码失败：%v", err)
	}
	code := mockCodeOf(t, sms, "13900139000")
	if _, err := svc.Register("13900139000", code, testPass, "", "ABCD2345"); err != nil {
		t.Fatalf("首次注册应成功：%v", err)
	}
	if err := NewInviteService().Check("ABCD2345"); !errors.Is(err, ErrInviteUsed) {
		t.Fatalf("注册成功后码应作废，实际 %v", err)
	}

	// ③ 拿一张新码：手机号重复（另一个已注册号）⇒ 码不该被烧 ✓
	seedInvite(t, "EFGH6789")
	sms3 := testMockSMS()
	svc3 := NewAuthService(testInviteAuthConfig(true), sms3, database.DB())
	if err := sms3.SendCode("13900139000", "127.0.0.1"); err != nil {
		t.Fatalf("发码失败：%v", err)
	}
	code3 := mockCodeOf(t, sms3, "13900139000")
	if _, err := svc3.Register("13900139000", code3, testPass, "", "EFGH6789"); !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("重复手机号应返回 ErrPhoneTaken，实际 %v", err)
	}
	if err := NewInviteService().Check("EFGH6789"); err != nil {
		t.Fatalf("手机号重复时不该消耗邀请码，实际 %v", err)
	}
}

func TestInviteGenerateShapeAndUniqueness(t *testing.T) {
	setupUsageTestDB(t)
	svc := NewInviteService()
	items, err := svc.Generate("单测批次", 5)
	if err != nil {
		t.Fatalf("生成失败：%v", err)
	}
	if len(items) != 5 {
		t.Fatalf("应生成 5 张，实际 %d", len(items))
	}
	seen := map[string]bool{}
	for _, it := range items {
		if len(it.Code) != InviteCodeLen {
			t.Fatalf("长度应为 %d，实际 %q", InviteCodeLen, it.Code)
		}
		if strings.Trim(it.Code, inviteAlphabet) != "" {
			t.Fatalf("出现了字母表外的字符：%q", it.Code)
		}
		if seen[it.Code] {
			t.Fatalf("本批内出现重复码：%q", it.Code)
		}
		seen[it.Code] = true
	}

	// 可用数 + 列表 ✓
	n, err := svc.CountUsable()
	if err != nil || n != 5 {
		t.Fatalf("可用码应为 5，实际 %d（err=%v）", n, err)
	}
	list, err := svc.List(true, 10)
	if err != nil || len(list) != 5 {
		t.Fatalf("列表应有 5 条，实际 %d（err=%v）", len(list), err)
	}
	// 停用一张 → 可用数-1 ✓
	if err := svc.SetEnabled(items[0].Code, false); err != nil {
		t.Fatalf("停用失败：%v", err)
	}
	if n2, _ := svc.CountUsable(); n2 != 4 {
		t.Fatalf("停用后可用数应为 4，实际 %d", n2)
	}
}

// 前端据此决定"要不要显示/要求邀请码" ✓
func TestRegisterOptionsSurfacesRequireInvite(t *testing.T) {
	on := NewAuthService(testInviteAuthConfig(true), testMockSMS(), nil).RegisterOptions()
	if !on.RequireInviteCode {
		t.Fatal("开启时 requireInviteCode 应为 true（前端据此显示邀请码栏 ✓）")
	}
	off := NewAuthService(testAuthConfig(true), testMockSMS(), nil).RegisterOptions()
	if off.RequireInviteCode {
		t.Fatal("未开启时应为 false（不影响既有部署 ✓）")
	}
}
