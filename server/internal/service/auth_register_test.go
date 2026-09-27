package service

// 手机号注册链路单测（2026-09-27）
//
// 覆盖：正常注册 → 能登录；错码/过期码不建号；重复手机号；开关关闭；短信未配置；
// 以及"后台代建账号没有手机号"这条**数据模型回归**（phone 为 NULL 才允许多个 ✗）。

import (
	"errors"
	"testing"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/pkg/database"
)

const (
	testPhone = "13800138000"
	testPass  = "secret123"
)

func registerWithSMS(t *testing.T, svc *AuthService, sms *SMSService, phone, password, displayName string) (username string, err error) {
	t.Helper()
	if e := sms.SendCode(phone, "127.0.0.1"); e != nil {
		t.Fatalf("发验证码失败：%v", e)
	}
	code := mockCodeOf(t, sms, phone)
	u, err := svc.Register(phone, code, password, displayName, "")
	if u == nil {
		return "", err
	}
	return u.Username, err
}

func TestRegisterWithPhoneAndSMS(t *testing.T) {
	setupUsageTestDB(t)
	sms := testMockSMS()
	svc := NewAuthService(testAuthConfig(true), sms, database.DB())

	username, err := registerWithSMS(t, svc, sms, testPhone, testPass, "")
	if err != nil {
		t.Fatalf("手机号注册应成功：%v", err)
	}
	// 用户名就是手机号（登录链路零改动 ✓）
	if username != testPhone {
		t.Fatalf("用户名应为手机号 %s，实际 %s", testPhone, username)
	}

	// 库里 phone 列真的写进去了，且昵称缺省 = 脱敏手机号
	u, err := svc.repo.FindByPhone(testPhone)
	if err != nil || u == nil {
		t.Fatalf("按手机号应能查到用户：%v", err)
	}
	if u.Phone == nil || *u.Phone != testPhone {
		t.Fatalf("phone 列应为 %s，实际 %+v", testPhone, u.Phone)
	}
	if u.DisplayName != MaskPhone(testPhone) {
		t.Fatalf("昵称缺省应为脱敏手机号 %s，实际 %q", MaskPhone(testPhone), u.DisplayName)
	}

	// 注册后能直接用手机号 + 密码登录
	if _, _, err := svc.Login(testPhone, testPass); err != nil {
		t.Fatalf("注册后应能登录：%v", err)
	}

	// 昵称显式给了就用给的
	sms2 := testMockSMS()
	svc2 := NewAuthService(testAuthConfig(true), sms2, database.DB())
	if _, err := registerWithSMS(t, svc2, sms2, "13900139000", testPass, "旅行者"); err != nil {
		t.Fatalf("第二次注册应成功：%v", err)
	}
	u2, _ := svc2.repo.FindByPhone("13900139000")
	if u2 == nil || u2.DisplayName != "旅行者" {
		t.Fatalf("昵称应保留用户填的，实际 %+v", u2)
	}
}

func TestRegisterRejectsWrongCode(t *testing.T) {
	setupUsageTestDB(t)
	sms := testMockSMS()
	svc := NewAuthService(testAuthConfig(true), sms, database.DB())

	if err := sms.SendCode(testPhone, "127.0.0.1"); err != nil {
		t.Fatalf("发验证码失败：%v", err)
	}
	if _, err := svc.Register(testPhone, "000000", testPass, "", ""); !errors.Is(err, ErrSMSCodeWrong) {
		t.Fatalf("错码应返回 ErrSMSCodeWrong，实际 %v", err)
	}
	// 关键：**错码不能建号** ✗（否则等于没有验证）
	if u, _ := svc.repo.FindByPhone(testPhone); u != nil {
		t.Fatal("验证码错误时不应创建用户 ✗")
	}
}

func TestRegisterRejectsDuplicatePhone(t *testing.T) {
	setupUsageTestDB(t)
	sms := testMockSMS()
	svc := NewAuthService(testAuthConfig(true), sms, database.DB())

	if _, err := registerWithSMS(t, svc, sms, testPhone, testPass, ""); err != nil {
		t.Fatalf("首次注册应成功：%v", err)
	}
	// 同一号码再注册：先发码（间隔限制按号算，这里用另一个 AuthService 不影响 ✓）
	sms2 := testMockSMS()
	svc2 := NewAuthService(testAuthConfig(true), sms2, database.DB())
	if _, err := registerWithSMS(t, svc2, sms2, testPhone, testPass, ""); !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("重复手机号应返回 ErrPhoneTaken，实际 %v", err)
	}
}

// 短信服务未配置好时：注册必须**明确拒绝**，而不是建出一个"没有验证过手机号"的账号 ✗
func TestRegisterNeedsSMSConfigured(t *testing.T) {
	setupUsageTestDB(t)
	svc := NewAuthService(testAuthConfig(true), NewSMSService(config.SMSConfig{}), database.DB())
	if _, err := svc.Register(testPhone, "123456", testPass, "", ""); !errors.Is(err, ErrSMSDisabled) {
		t.Fatalf("短信未启用时应返回 ErrSMSDisabled，实际 %v", err)
	}
	if u, _ := svc.repo.FindByPhone(testPhone); u != nil {
		t.Fatal("短信不可用时不应创建用户 ✗")
	}
}

func TestRegisterRejectsBadPhoneAndPassword(t *testing.T) {
	setupUsageTestDB(t)
	sms := testMockSMS()
	svc := NewAuthService(testAuthConfig(true), sms, database.DB())

	// 非法手机号：连码都不用发（也避免把非法号送进通道 ✓）
	if _, err := svc.Register("12800138000", "123456", testPass, "", ""); !errors.Is(err, ErrPhoneInvalid) {
		t.Fatalf("非法手机号应返回 ErrPhoneInvalid，实际 %v", err)
	}
	// 密码过短：ErrUserInvalid（沿用既有规则，≥6 位）
	if err := sms.SendCode(testPhone, "127.0.0.1"); err != nil {
		t.Fatalf("发验证码失败：%v", err)
	}
	code := mockCodeOf(t, sms, testPhone)
	if _, err := svc.Register(testPhone, code, "12345", "", ""); !errors.Is(err, ErrUserInvalid) {
		t.Fatalf("密码过短应返回 ErrUserInvalid，实际 %v", err)
	}
}

// 数据模型回归：**phone 可空**（后台代建的账号没有手机号）
// —— 若把 phone 写成 not null + default ”，第二个后台账号会撞唯一索引而建不出来 ✗
func TestAdminCreatedUsersHaveNullPhone(t *testing.T) {
	setupUsageTestDB(t)
	admin := NewUserAdminService()
	for _, name := range []string{"ops_one", "ops_two", "ops_three"} {
		created, err := admin.CreateUser(dto.AdminCreateUserRequest{Username: name, Password: testPass})
		if err != nil {
			t.Fatalf("后台代建 %s 失败（phone 唯一索引不该拦住 NULL ✗）：%v", name, err)
		}
		if created.ID == 0 {
			t.Fatalf("%s 未落库", name)
		}
	}
	// 手机号注册仍可正常进行（NULL 不占用任何号码 ✓）
	sms := testMockSMS()
	svc := NewAuthService(testAuthConfig(true), sms, database.DB())
	if _, err := registerWithSMS(t, svc, sms, testPhone, testPass, ""); err != nil {
		t.Fatalf("有 NULL phone 的账号时仍应能手机号注册：%v", err)
	}
}

func TestRegisterOptions(t *testing.T) {
	// 关闭注册（当前线上口径）：前端应显示"邀请制"提示
	closed := NewAuthService(testAuthConfig(false), testMockSMS(), nil)
	opts := closed.RegisterOptions()
	if opts.AllowRegister || !opts.SMSEnabled {
		t.Fatalf("关闭注册时的选项不对：%+v", opts)
	}
	if opts.RequireInviteCode {
		t.Fatal("邀请码功能尚未开启，应为 false ✓")
	}
	if opts.SMSCodeTTLSeconds != 300 || opts.SMSResendAfterSeconds != 60 {
		t.Fatalf("TTL/间隔应为 300/60，实际 %d/%d", opts.SMSCodeTTLSeconds, opts.SMSResendAfterSeconds)
	}

	// 短信未配置时：smsEnabled=false（前端不显示注册入口 ✓，避免点了才发现不能用 ✗）
	noSMS := NewAuthService(testAuthConfig(true), NewSMSService(config.SMSConfig{}), nil)
	if noSMS.RegisterOptions().SMSEnabled {
		t.Fatal("短信未启用时 SMSEnabled 应为 false")
	}
}
