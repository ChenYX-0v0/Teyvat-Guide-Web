package service

import (
	"strings"
	"testing"

	"teyvat-guide/server/internal/dto"
)

// V2.1：命中角色的权威词条必须带上分级与主词条，并明确标注这是唯一依据
func TestBuildAssetSummary(t *testing.T) {
	refs := []CharacterStatRef{
		{
			Name: "胡桃",
			SubStats: []dto.StatWeightEntry{
				{Name: "暴击率", Weight: 10, Tier: "primary"},
				{Name: "暴击伤害", Weight: 10, Tier: "primary"},
				{Name: "生命值%", Weight: 6, Tier: "secondary"},
			},
			MainStats: []StatSlotRef{
				{Slot: "时之沙", Stat: "生命值%"},
				{Slot: "空之杯", Stat: "火元素伤害加成"},
			},
		},
	}

	got := BuildAssetSummary(refs)
	for _, want := range []string{
		"【游戏资料】",
		"胡桃",
		"暴击率(primary)",
		"暴击伤害(primary)",
		"生命值%(secondary)",
		"推荐主词条",
		"时之沙 生命值%",
		"空之杯 火元素伤害加成",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("资料文本缺少 %q：\n%s", want, got)
		}
	}
}

// V2.3：命中但库里没资料的角色，必须显式说明「暂无资料」，否则模型会用记忆编数值
func TestBuildAssetSummaryNoData(t *testing.T) {
	got := BuildAssetSummary([]CharacterStatRef{{Name: "某新角色"}})

	if !strings.Contains(got, "某新角色") {
		t.Fatalf("应带上角色名：%s", got)
	}
	if !strings.Contains(got, "本服暂无该角色的资料") {
		t.Fatalf("必须显式标注暂无资料：%s", got)
	}
}

func TestBuildAssetSummaryEmpty(t *testing.T) {
	if got := BuildAssetSummary(nil); got != "" {
		t.Fatalf("没有命中角色时不应推送资料：%q", got)
	}
}

// 命中角色过多时按上限截断，避免长提问把上下文塞满
func TestBuildAssetSummaryCapsCharacters(t *testing.T) {
	refs := []CharacterStatRef{
		{Name: "甲", SubStats: []dto.StatWeightEntry{{Name: "暴击率", Weight: 1, Tier: "primary"}}},
		{Name: "乙", SubStats: []dto.StatWeightEntry{{Name: "暴击率", Weight: 1, Tier: "primary"}}},
		{Name: "丙", SubStats: []dto.StatWeightEntry{{Name: "暴击率", Weight: 1, Tier: "primary"}}},
	}

	got := BuildAssetSummary(refs)

	if n := strings.Count(got, "\n- "); n != maxReferenceCharacters {
		t.Fatalf("期望只保留 %d 个角色，实际 %d 个：\n%s", maxReferenceCharacters, n, got)
	}
	if strings.Contains(got, "丙") {
		t.Fatalf("超出上限的角色不应出现：\n%s", got)
	}
}
