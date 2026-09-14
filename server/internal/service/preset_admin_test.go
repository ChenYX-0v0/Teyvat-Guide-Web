package service

// 预设问答的后台维护（新建 / 编辑 / 上下架）：
// 口径是「下架只改 enabled，不删数据」——运营随时改、随时撤，C 端立刻生效。

import (
	"errors"
	"strings"
	"testing"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/repository"
)

func TestPresetQAAdminCRUD(t *testing.T) {
	setupUsageTestDB(t)
	svc := NewPresetQAService()
	repo := repository.NewPresetQARepository()

	all, err := svc.ListAll()
	if err != nil {
		t.Fatalf("后台列表失败：%v", err)
	}
	seedCount := len(all.Items)
	if seedCount == 0 {
		t.Fatal("首次启动应有种子数据")
	}

	// 新建：Enabled 缺省为 true；字段去首尾空白
	created, err := svc.Create(dto.PresetQASaveRequest{
		SceneKey: "  event  ",
		Question: "  这次活动值得抽吗？  ",
		Answer:   "  看你的原石储备。  ",
		Sort:     5,
	})
	if err != nil {
		t.Fatalf("新建失败：%v", err)
	}
	if !created.Enabled {
		t.Fatal("新建缺省应为启用")
	}
	if created.SceneKey != "event" || created.Question != "这次活动值得抽吗？" || created.Answer != "看你的原石储备。" {
		t.Fatalf("字段未归一化：%+v", created)
	}

	// C 端立刻能看到
	cList, err := svc.List()
	if err != nil {
		t.Fatalf("C 端列表失败：%v", err)
	}
	if len(cList.Items) != seedCount+1 {
		t.Fatalf("C 端应多 1 条：期望 %d，实际 %d", seedCount+1, len(cList.Items))
	}

	// 下架：C 端不再下发、后台仍可见、数据不删
	disabled := false
	if _, err := svc.Update(created.ID, dto.PresetQASaveRequest{
		SceneKey: created.SceneKey, Question: created.Question, Answer: created.Answer,
		Sort: created.Sort, Enabled: &disabled,
	}); err != nil {
		t.Fatalf("下架失败：%v", err)
	}
	cAfter, err := svc.List()
	if err != nil {
		t.Fatalf("C 端列表失败：%v", err)
	}
	if len(cAfter.Items) != seedCount {
		t.Fatalf("下架后 C 端应回到 %d 条，实际 %d", seedCount, len(cAfter.Items))
	}
	adminAfter, err := svc.ListAll()
	if err != nil {
		t.Fatalf("后台列表失败：%v", err)
	}
	if len(adminAfter.Items) != seedCount+1 {
		t.Fatalf("后台应仍能看到（含已下架）：期望 %d，实际 %d", seedCount+1, len(adminAfter.Items))
	}

	// 只改文案（Enabled 缺省 = 保持原值，仍是下架状态）
	updated, err := svc.Update(created.ID, dto.PresetQASaveRequest{
		SceneKey: created.SceneKey, Question: "改过的提问", Answer: created.Answer, Sort: 1,
	})
	if err != nil {
		t.Fatalf("更新失败：%v", err)
	}
	if updated.Question != "改过的提问" {
		t.Fatalf("文案未更新：%+v", updated)
	}
	if updated.Enabled {
		t.Fatalf("Enabled 缺省应保持原值（下架），实际 %+v", updated)
	}
	if row, err := repo.FindByID(created.ID); err != nil || row == nil {
		t.Fatalf("更新不应删除数据：row=%v err=%v", row, err)
	}
}

func TestPresetQAAdminValidation(t *testing.T) {
	setupUsageTestDB(t)
	svc := NewPresetQAService()

	cases := []struct {
		name string
		req  dto.PresetQASaveRequest
	}{
		{"空提问", dto.PresetQASaveRequest{Question: "   ", Answer: "有回答"}},
		{"空回答", dto.PresetQASaveRequest{Question: "有提问", Answer: "  "}},
		{"提问超长", dto.PresetQASaveRequest{Question: strings.Repeat("问", 201), Answer: "有回答"}},
		{"回答超长", dto.PresetQASaveRequest{Question: "有提问", Answer: strings.Repeat("答", 4001)}},
		{"分组超长", dto.PresetQASaveRequest{SceneKey: strings.Repeat("s", 33), Question: "有提问", Answer: "有回答"}},
		{"排序越界", dto.PresetQASaveRequest{Question: "有提问", Answer: "有回答", Sort: 10000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(tc.req); !errors.Is(err, ErrPresetInvalid) {
				t.Fatalf("期望 ErrPresetInvalid，实际 %v", err)
			}
		})
	}

	// 不存在的 ID → 404 语义（不是 400/500）
	_, err := svc.Update(999999, dto.PresetQASaveRequest{Question: "有提问", Answer: "有回答"})
	if !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("期望 ErrPresetNotFound，实际 %v", err)
	}
}
