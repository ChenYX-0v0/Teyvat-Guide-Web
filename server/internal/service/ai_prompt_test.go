package service

// 用 sqlite 跑通提示词读取：同场景取「启用且版本最高」、scenes 解析与过滤、未知场景不返回

import (
	"path/filepath"
	"strings"
	"testing"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
)

func setupAIPromptService(t *testing.T) *AIPromptService {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "ai_prompt_test.db")
	if err := database.Init(config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("初始化 sqlite 失败：%v", err)
	}
	db := database.DB()
	if err := db.AutoMigrate(&model.AIPrompt{}); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err != nil {
			return
		}
		if err := sqlDB.Close(); err != nil {
			t.Logf("关闭测试数据库失败：%v", err)
		}
	})

	rows := []model.AIPrompt{
		{SceneKey: "paimon_base", Content: "base v1", Version: 1, Enabled: true},
		{SceneKey: "paimon_rules", Content: "rules v1", Version: 1, Enabled: true},
		{SceneKey: "paimon_rules", Content: "rules v2", Version: 2, Enabled: true},
		// 版本最高但未启用：灰度中的新版本不该胜出
		{SceneKey: "paimon_rules", Content: "rules v3", Version: 3, Enabled: true},
		{SceneKey: "demo_scene", Content: "demo v1", Version: 1, Enabled: true},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("写入提示词失败：%v", err)
		}
	}
	// Enabled 字段带 default:true，GORM 建表时会跳过 false 零值，因此显式转为未启用
	if err := db.Model(&model.AIPrompt{}).
		Where("scene_key = ? AND version = ?", "paimon_rules", 3).
		Update("enabled", false).Error; err != nil {
		t.Fatalf("关闭版本失败：%v", err)
	}

	return NewAIPromptService()
}

func TestAIPromptCurrentPicksHighestEnabledVersion(t *testing.T) {
	svc := setupAIPromptService(t)

	data, err := svc.Current([]string{"paimon_rules"})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("期望 1 条，实际 %d：%+v", len(data.Items), data.Items)
	}
	item := data.Items[0]
	if item.SceneKey != "paimon_rules" || item.Version != 2 || item.Content != "rules v2" {
		t.Fatalf("应返回启用中的最高版本 v2，实际 %+v", item)
	}
}

func TestAIPromptCurrentAllScenes(t *testing.T) {
	svc := setupAIPromptService(t)

	data, err := svc.Current(nil)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(data.Items) != 3 {
		t.Fatalf("期望 3 个场景，实际 %d：%+v", len(data.Items), data.Items)
	}
	keys := make([]string, 0, len(data.Items))
	for _, it := range data.Items {
		keys = append(keys, it.SceneKey)
	}
	if got := strings.Join(keys, ","); got != "demo_scene,paimon_base,paimon_rules" {
		t.Fatalf("场景顺序应按 scene_key 升序：%s", got)
	}
}

func TestAIPromptCurrentScenesParsing(t *testing.T) {
	svc := setupAIPromptService(t)

	// 空白 / 重复 / 空串都要被归一化，且不会因为空串而"放行全部"
	data, err := svc.Current([]string{" paimon_base ", "paimon_base", "", "  "})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(data.Items) != 1 || data.Items[0].SceneKey != "paimon_base" {
		t.Fatalf("scenes 归一化不符：%+v", data.Items)
	}

	// 未知场景不返回（AI 服务侧按未命中处理）
	empty, err := svc.Current([]string{"not-exist"})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(empty.Items) != 0 {
		t.Fatalf("未知场景不应返回：%+v", empty.Items)
	}
}
