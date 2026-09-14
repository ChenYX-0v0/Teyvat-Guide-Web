package service

// 邀请制（不公开部署）相关的两个入口：
//   1. 后台代建账号 UserAdminService.CreateUser —— 公开注册关闭时的唯一开户入口；
//   2. auth.allow_register 开关 —— 关闭后 /api/auth/register 拒绝公开注册。

import (
	"errors"
	"testing"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/pkg/database"
)

func testAuthConfig(allowRegister bool) config.AuthConfig {
	return config.AuthConfig{
		JWTSecret:     "test-secret",
		JWTTTLHours:   1,
		AllowRegister: allowRegister,
		SeedUser:      "seed_admin",
		SeedPass:      "seed123456",
	}
}

func TestUserAdminCreateUser(t *testing.T) {
	setupUsageTestDB(t)
	svc := NewUserAdminService()

	created, err := svc.CreateUser(dto.AdminCreateUserRequest{
		Username: "  invited_user  ", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("创建失败：%v", err)
	}
	// 去空白 + displayName 缺省 = username
	if created.Username != "invited_user" || created.DisplayName != "invited_user" {
		t.Fatalf("字段未归一化：%+v", created)
	}
	if created.ID == 0 || created.CreatedAt == "" {
		t.Fatalf("应回传 id 与创建时间：%+v", created)
	}

	// 密码哈希写对了：后台建的账号能直接登录
	auth := NewAuthService(testAuthConfig(false), database.DB())
	if _, _, err := auth.Login("invited_user", "secret123"); err != nil {
		t.Fatalf("后台建的账号应能登录：%v", err)
	}

	// 重名 → ErrUsernameTaken（Handler 映射 409）
	if _, err := svc.CreateUser(dto.AdminCreateUserRequest{
		Username: "invited_user", Password: "secret123",
	}); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("期望 ErrUsernameTaken，实际 %v", err)
	}

	// 参数不合法 → ErrUserInvalid（Handler 映射 400）
	for name, req := range map[string]dto.AdminCreateUserRequest{
		"用户名过短": {Username: "ab", Password: "secret123"},
		"密码过短":  {Username: "ok_user", Password: "12345"},
	} {
		if _, err := svc.CreateUser(req); !errors.Is(err, ErrUserInvalid) {
			t.Fatalf("%s：期望 ErrUserInvalid，实际 %v", name, err)
		}
	}
}

func TestAuthRegisterRespectsAllowRegister(t *testing.T) {
	setupUsageTestDB(t)
	db := database.DB()

	// 不公开 / 邀请制：公开注册关闭
	closed := NewAuthService(testAuthConfig(false), db)
	if _, err := closed.Register("self_signup", "secret123", ""); !errors.Is(err, ErrRegisterClosed) {
		t.Fatalf("allow_register=false 时应拒绝公开注册，实际 %v", err)
	}
	// 但后台代建不受开关影响（否则邀请制部署没法开户）
	if _, err := NewUserAdminService().CreateUser(dto.AdminCreateUserRequest{
		Username: "admin_made", Password: "secret123",
	}); err != nil {
		t.Fatalf("关闭公开注册时后台仍应能建号：%v", err)
	}

	// 开放注册（默认行为，用于将来公开）
	open := NewAuthService(testAuthConfig(true), db)
	if _, err := open.Register("self_signup", "secret123", ""); err != nil {
		t.Fatalf("allow_register=true 时应可注册：%v", err)
	}
}
