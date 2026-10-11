package service

// 侧栏用户菜单带来的两个能力（2026-10-11）：
//   · 改昵称  PUT /api/auth/profile
//   · 改密码  POST /api/auth/password
//
// 用 sqlite 临时库，与既有单测同套路（setupUsageTestDB 见 usage_quota_test.go ✓）。
// 重点验的是**行为**而不是实现细节：改完密码后"新密码能登录、旧密码不能" ✓（真打 Login ✓）。

import (
	"errors"
	"strings"
	"testing"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/repository"
)

// newAuthServiceForTest 造一个只带仓储的 AuthService：
// 不碰短信通道、不跑种子账号（seedIfEmpty 在构造函数里，这里刻意不走它 ✓）。
// 字段未导出但**同包可直接构造** ✓（既有 terms_test.go 也是这么写的 ✓）。
func newAuthServiceForTest() *AuthService {
	return &AuthService{repo: repository.NewUserRepository()}
}

// TestUpdateDisplayNameTrimsLimitsAndClears 昵称：去空格 / 按字符限长 / 空串=清空 / 幂等 / 未知用户
func TestUpdateDisplayNameTrimsLimitsAndClears(t *testing.T) {
	setupUsageTestDB(t)
	svc := newAuthServiceForTest()
	repo := repository.NewUserRepository()

	u := &model.User{Username: "13800000000", PasswordHash: "x", DisplayName: "小明"}
	if err := repo.Create(u); err != nil {
		t.Fatalf("造账号失败：%v", err)
	}

	// ① 前后空格吃掉
	got, err := svc.UpdateDisplayName(u.ID, "  阿花  ")
	if err != nil {
		t.Fatalf("改昵称失败：%v", err)
	}
	if got.DisplayName != "阿花" {
		t.Fatalf("应去掉前后空格，实际 = %q", got.DisplayName)
	}
	// ② 真落库了（不只看内存返回值 ✓）
	if fresh, err := repo.FindByID(u.ID); err != nil || fresh == nil || fresh.DisplayName != "阿花" {
		t.Fatalf("库里昵称应为 阿花，err=%v", err)
	}

	// ③ 超长按**字符**计：24 上限 ⇒ 25 个汉字应被拒，且库里保持不变
	if _, err := svc.UpdateDisplayName(u.ID, strings.Repeat("字", displayNameMaxRunes+1)); !errors.Is(err, ErrUserInvalid) {
		t.Fatalf("超长昵称应返回 ErrUserInvalid，实际 = %v", err)
	}
	if fresh, _ := repo.FindByID(u.ID); fresh.DisplayName != "阿花" {
		t.Fatalf("被拒的请求不该改动库，实际 = %q", fresh.DisplayName)
	}
	// ④ 恰好等于上限 ⇒ 允许（边界值不走偏 ✓）
	if _, err := svc.UpdateDisplayName(u.ID, strings.Repeat("字", displayNameMaxRunes)); err != nil {
		t.Fatalf("恰好 %d 个字符应允许：%v", displayNameMaxRunes, err)
	}

	// ⑤ 空串 / 全空格 = 清空昵称（前端会回落到用户名展示 ✓）
	got, err = svc.UpdateDisplayName(u.ID, "   ")
	if err != nil || got.DisplayName != "" {
		t.Fatalf("空串应清空昵称，实际 = %q err=%v", got.DisplayName, err)
	}
	// ⑥ 幂等：再清一次也不报错
	if _, err := svc.UpdateDisplayName(u.ID, ""); err != nil {
		t.Fatalf("重复清空应幂等：%v", err)
	}

	// ⑦ 未知用户
	if _, err := svc.UpdateDisplayName(99999, "x"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("未知用户应返回 ErrUserNotFound，实际 = %v", err)
	}
}

// TestChangePasswordVerifiesOldAndRotatesHash 改密：旧密码校验 → 同一套强度规则 → 新密码生效、旧密码失效
func TestChangePasswordVerifiesOldAndRotatesHash(t *testing.T) {
	setupUsageTestDB(t)
	svc := newAuthServiceForTest()
	repo := repository.NewUserRepository()

	// 用 buildUser 造账号 ⇒ 哈希方式与线上**完全同源** ✓
	u, err := buildUser("13800000001", "oldpass123", "小测")
	if err != nil {
		t.Fatalf("buildUser 失败：%v", err)
	}
	if err := repo.Create(u); err != nil {
		t.Fatalf("造账号失败：%v", err)
	}

	// ① 旧密码不对 ⇒ ErrOldPasswordWrong（Handler 映射 400 ✓ 而不是 401 ✗）
	if err := svc.ChangePassword(u.ID, "wrongpass", "newpass456"); !errors.Is(err, ErrOldPasswordWrong) {
		t.Fatalf("旧密码错应返回 ErrOldPasswordWrong，实际 = %v", err)
	}
	// ② 与旧密码相同 ⇒ ErrPasswordSame
	if err := svc.ChangePassword(u.ID, "oldpass123", "oldpass123"); !errors.Is(err, ErrPasswordSame) {
		t.Fatalf("新旧相同应返回 ErrPasswordSame，实际 = %v", err)
	}
	// ③ 新密码太短 ⇒ ErrUserInvalid（与注册**共用同一套规则** ✓）
	if err := svc.ChangePassword(u.ID, "oldpass123", "123"); !errors.Is(err, ErrUserInvalid) {
		t.Fatalf("太短应返回 ErrUserInvalid，实际 = %v", err)
	}
	// ④ 以上三次失败都不该动库：旧密码仍然能登录 ✓
	if _, _, err := svc.Login("13800000001", "oldpass123"); err != nil {
		t.Fatalf("失败的改密不该影响原密码，登录却失败：%v", err)
	}

	// ⑤ 正常改密
	if err := svc.ChangePassword(u.ID, "oldpass123", "newpass456"); err != nil {
		t.Fatalf("改密应成功：%v", err)
	}
	// ⑥ 行为验证：新密码能登录、旧密码不能（真打 Login ✓，不是只看哈希字段 ✓）
	if _, _, err := svc.Login("13800000001", "newpass456"); err != nil {
		t.Fatalf("新密码应能登录：%v", err)
	}
	if _, _, err := svc.Login("13800000001", "oldpass123"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("旧密码应登录失败，实际 = %v", err)
	}

	// ⑦ 未知用户
	if err := svc.ChangePassword(99999, "a123456", "b123456"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("未知用户应返回 ErrUserNotFound，实际 = %v", err)
	}
}
