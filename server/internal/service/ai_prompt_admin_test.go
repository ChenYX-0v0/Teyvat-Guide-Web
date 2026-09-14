package service

// 提示词后台维护：版本号服务端分配、新增即生效、停用即回滚到上一启用版本、
// "先入库不发"（enabled=false）能真的落成未启用、参数校验与 404 语义。

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
)

// setupAIPromptAdminDB 起一个只含 ai_prompts 的 sqlite：rules 有 v2 / v3 两个启用版本
func setupAIPromptAdminDB(t *testing.T) {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "ai_prompt_admin_test.db")
	if err := database.Init(config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("初始化 sqlite 失败：%v", err)
	}
	db := database.DB()
	if err := db.AutoMigrate(&model.AIPrompt{}); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	rows := []model.AIPrompt{
		{SceneKey: "paimon_rules", Content: "rules v2", Version: 2, Enabled: true, Remark: "v2"},
		{SceneKey: "paimon_rules", Content: "rules v3", Version: 3, Enabled: true, Remark: "v3"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("写入提示词失败：%v", err)
		}
	}
}

// findItem 从后台列表里按 场景 + 版本 找到那一行
func findItem(t *testing.T, list *dto.AIPromptAdminList, sceneKey string, version int) dto.AIPromptAdminItem {
	t.Helper()
	for _, it := range list.Items {
		if it.SceneKey == sceneKey && it.Version == version {
			return it
		}
	}
	t.Fatalf("列表里没有 %s v%d：%+v", sceneKey, version, list.Items)
	return dto.AIPromptAdminItem{}
}

// currentVersion 读"AI 服务实际会拿到的版本"
func currentVersion(t *testing.T, sceneKey string) int {
	t.Helper()
	data, err := NewAIPromptService().Current([]string{sceneKey})
	if err != nil {
		t.Fatalf("读取当前生效版本失败：%v", err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("期望 1 条生效提示词，实际 %d", len(data.Items))
	}
	return data.Items[0].Version
}

func TestAIPromptAdminCreateAssignsNextVersion(t *testing.T) {
	setupAIPromptAdminDB(t)
	svc := NewAIPromptAdminService()

	created, err := svc.Create(dto.AIPromptCreateRequest{
		SceneKey: " paimon_rules ", Content: "  rules v4  ", Remark: " 去掉 Markdown ",
	})
	if err != nil {
		t.Fatalf("新增失败：%v", err)
	}
	if created.Version != 4 {
		t.Fatalf("版本号应由服务端分配为 4，实际 %d", created.Version)
	}
	if !created.Enabled || !created.Effective {
		t.Fatalf("新增且启用应立即可生效：%+v", created)
	}
	if created.SceneKey != "paimon_rules" || created.Content != "rules v4" || created.Remark != "去掉 Markdown" {
		t.Fatalf("字段未归一化：%+v", created)
	}
	if !created.Served {
		t.Fatal("paimon_rules 属于 AI 服务读取的场景，served 应为 true")
	}
	if got := currentVersion(t, "paimon_rules"); got != 4 {
		t.Fatalf("AI 服务侧应立刻拿到 v4，实际 v%d", got)
	}
}

func TestAIPromptAdminCreateDisabledDoesNotTakeEffect(t *testing.T) {
	setupAIPromptAdminDB(t)
	svc := NewAIPromptAdminService()

	disabled := false
	created, err := svc.Create(dto.AIPromptCreateRequest{
		SceneKey: "paimon_rules", Content: "rules v4 草稿", Enabled: &disabled,
	})
	if err != nil {
		t.Fatalf("新增失败：%v", err)
	}
	if created.Enabled {
		t.Fatal("enabled=false 应落成未启用（GORM 零值陷阱回归）")
	}
	if created.Effective {
		t.Fatal("未启用版本不应是生效版本")
	}
	if got := currentVersion(t, "paimon_rules"); got != 3 {
		t.Fatalf("未启用版本不应生效，仍应是 v3，实际 v%d", got)
	}
}

func TestAIPromptAdminSetEnabledRollsBack(t *testing.T) {
	setupAIPromptAdminDB(t)
	svc := NewAIPromptAdminService()

	list, err := svc.ListAll()
	if err != nil {
		t.Fatalf("后台列表失败：%v", err)
	}
	v2 := findItem(t, list, "paimon_rules", 2)
	v3 := findItem(t, list, "paimon_rules", 3)
	if v2.Effective || !v3.Effective {
		t.Fatalf("初始生效版本应为 v3：v2.effective=%v v3.effective=%v", v2.Effective, v3.Effective)
	}

	// 停用 v3 = 回滚：生效版本自动回落到 v2，内容一行未改
	updated, err := svc.SetEnabled(v3.ID, false)
	if err != nil {
		t.Fatalf("停用失败：%v", err)
	}
	if updated.Enabled || updated.Effective {
		t.Fatalf("停用后不应仍是生效版本：%+v", updated)
	}
	if updated.Content != "rules v3" {
		t.Fatalf("启停不应改动内容：%+v", updated)
	}
	if got := currentVersion(t, "paimon_rules"); got != 2 {
		t.Fatalf("回滚后应生效 v2，实际 v%d", got)
	}

	// 再启用 v3 = 前进回来
	back, err := svc.SetEnabled(v3.ID, true)
	if err != nil {
		t.Fatalf("重新启用失败：%v", err)
	}
	if !back.Effective {
		t.Fatalf("重新启用后应成为生效版本：%+v", back)
	}
	if got := currentVersion(t, "paimon_rules"); got != 3 {
		t.Fatalf("重新启用后应生效 v3，实际 v%d", got)
	}

	// 后台列表仍能看到全部版本（不做物理删除）
	after, err := svc.ListAll()
	if err != nil {
		t.Fatalf("后台列表失败：%v", err)
	}
	if len(after.Items) != len(list.Items) {
		t.Fatalf("启停不应增删行：%d -> %d", len(list.Items), len(after.Items))
	}
}

func TestAIPromptAdminSceneNotServedIsFlagged(t *testing.T) {
	setupAIPromptAdminDB(t)
	svc := NewAIPromptAdminService()

	created, err := svc.Create(dto.AIPromptCreateRequest{
		SceneKey: "request_extra", Content: "每请求补充指令",
	})
	if err != nil {
		t.Fatalf("新增失败：%v", err)
	}
	if created.Served {
		t.Fatal("不在 AI 服务白名单里的场景 served 应为 false，管理页要据此提示")
	}

	list, err := svc.ListAll()
	if err != nil {
		t.Fatalf("后台列表失败：%v", err)
	}
	want := "paimon_base,paimon_rules,paimon_fewshot"
	if got := strings.Join(list.ServedScenes, ","); got != want {
		t.Fatalf("白名单应回给前端做提示：期望 %s，实际 %s", want, got)
	}
}

func TestAIPromptAdminValidation(t *testing.T) {
	setupAIPromptAdminDB(t)
	svc := NewAIPromptAdminService()

	cases := []struct {
		name string
		req  dto.AIPromptCreateRequest
	}{
		{"空场景键", dto.AIPromptCreateRequest{Content: "有内容"}},
		{"场景键大写", dto.AIPromptCreateRequest{SceneKey: "Paimon_Rules", Content: "有内容"}},
		{"场景键带横线", dto.AIPromptCreateRequest{SceneKey: "paimon-rules", Content: "有内容"}},
		{"场景键数字开头", dto.AIPromptCreateRequest{SceneKey: "1rules", Content: "有内容"}},
		{"场景键超长", dto.AIPromptCreateRequest{SceneKey: "p" + strings.Repeat("a", 64), Content: "有内容"}},
		{"空内容", dto.AIPromptCreateRequest{SceneKey: "paimon_rules", Content: "   "}},
		{"内容超长", dto.AIPromptCreateRequest{SceneKey: "paimon_rules", Content: strings.Repeat("字", 20001)}},
		{"备注超长", dto.AIPromptCreateRequest{SceneKey: "paimon_rules", Content: "有内容", Remark: strings.Repeat("备", 201)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(tc.req); !errors.Is(err, ErrAIPromptInvalid) {
				t.Fatalf("期望 ErrAIPromptInvalid，实际 %v", err)
			}
		})
	}

	// 不存在的 ID → 404 语义（不是 400 / 500）
	if _, err := svc.SetEnabled(999999, true); !errors.Is(err, ErrAIPromptNotFound) {
		t.Fatalf("期望 ErrAIPromptNotFound，实际 %v", err)
	}
}
