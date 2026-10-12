package service

// 查询即绑定（2026-10-12 口径 A）单测：匿名不绑 / 首次绑 / 幂等 / 改绑 / 解绑后回绑
// 用例沿用既有 sqlite 套路（setupUsageTestDB 已迁 users ✓）；不触上游（AutoBind 刻意不复查 Enka ✓）。

import (
	"testing"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/repository"
)

func newAutoBindSvc() *PlayerService {
	return &PlayerService{
		repo:     repository.NewPlayerRepository(),
		userRepo: repository.NewUserRepository(),
	}
}

func seedAutoBindUser(t *testing.T, id uint) {
	t.Helper()
	if err := database.DB().Create(&model.User{
		ID: id, Username: "autobind_user", PasswordHash: "x", Status: model.UserStatusActive,
	}).Error; err != nil {
		t.Fatalf("建用户失败：%v", err)
	}
}

func TestAutoBindOnQuery(t *testing.T) {
	setupUsageTestDB(t)
	svc := newAutoBindSvc()

	// ① 未登录（userID=0）⇒ **不绑、不报错** ✓（匿名查询行为必须不变 ✓）
	if changed, err := svc.AutoBind(0, "197794747"); changed || err != nil {
		t.Fatalf("匿名不该绑定：changed=%v err=%v", changed, err)
	}
	// 空 UID 同理（handler 已经先做格式校验，这里只是双保险 ✓）
	if changed, err := svc.AutoBind(1, ""); changed || err != nil {
		t.Fatalf("空 UID 不该绑定：changed=%v err=%v", changed, err)
	}

	seedAutoBindUser(t, 1)

	// ② 首次查询 ⇒ 绑定成功 ✓，且写回默认区服 ✓
	changed, err := svc.AutoBind(1, "197794747")
	if err != nil {
		t.Fatalf("首次绑定失败：%v", err)
	}
	if !changed {
		t.Fatal("首次查询应产生绑定（changed=true）")
	}
	uid, server, err := svc.Binding(1)
	if err != nil {
		t.Fatalf("读绑定失败：%v", err)
	}
	if uid != "197794747" || server != DefaultGenshinServer {
		t.Fatalf("绑定结果不符：uid=%q server=%q", uid, server)
	}

	// ③ 同一个 UID 再查 ⇒ **幂等**：不写库、changed=false ✓
	changed, err = svc.AutoBind(1, "197794747")
	if err != nil || changed {
		t.Fatalf("同 UID 应幂等（changed=false）：changed=%v err=%v", changed, err)
	}

	// ④ 查**别的** UID ⇒ **改绑**（口径 A 的代价，用户已确认 ✓），且区服沿用原值（不被清空 ✗）
	changed, err = svc.AutoBind(1, "123456789")
	if err != nil {
		t.Fatalf("改绑失败：%v", err)
	}
	if !changed {
		t.Fatal("换 UID 应改绑（changed=true）")
	}
	uid, server, _ = svc.Binding(1)
	if uid != "123456789" {
		t.Fatalf("改绑后 uid 不符：%q", uid)
	}
	if server != DefaultGenshinServer {
		t.Fatalf("改绑不该清空区服：%q", server)
	}

	// ⑤ 解绑后再查 ⇒ 会重新绑上 ✓（并写回默认区服 ✓）
	if err := svc.Unbind(1); err != nil {
		t.Fatalf("解绑失败：%v", err)
	}
	changed, err = svc.AutoBind(1, "197794747")
	if err != nil || !changed {
		t.Fatalf("解绑后应重新绑定：changed=%v err=%v", changed, err)
	}
	uid, server, _ = svc.Binding(1)
	if uid != "197794747" || server != DefaultGenshinServer {
		t.Fatalf("解绑后回绑结果不符：uid=%q server=%q", uid, server)
	}
}
