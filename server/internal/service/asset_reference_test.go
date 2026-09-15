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

	got := BuildAssetSummary(refs, nil)
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
	got := BuildAssetSummary([]CharacterStatRef{{Name: "某新角色"}}, nil)

	if !strings.Contains(got, "某新角色") {
		t.Fatalf("应带上角色名：%s", got)
	}
	if !strings.Contains(got, "本服暂无该角色的资料") {
		t.Fatalf("必须显式标注暂无资料：%s", got)
	}
}

func TestBuildAssetSummaryEmpty(t *testing.T) {
	if got := BuildAssetSummary(nil, nil); got != "" {
		t.Fatalf("什么都没命中时不应推送资料：%q", got)
	}
}

// 命中角色过多时按上限截断，避免长提问把上下文塞满
func TestBuildAssetSummaryCapsCharacters(t *testing.T) {
	refs := []CharacterStatRef{
		{Name: "甲", SubStats: []dto.StatWeightEntry{{Name: "暴击率", Weight: 1, Tier: "primary"}}},
		{Name: "乙", SubStats: []dto.StatWeightEntry{{Name: "暴击率", Weight: 1, Tier: "primary"}}},
		{Name: "丙", SubStats: []dto.StatWeightEntry{{Name: "暴击率", Weight: 1, Tier: "primary"}}},
	}

	got := BuildAssetSummary(refs, nil)

	if n := strings.Count(got, "\n- "); n != maxReferenceCharacters {
		t.Fatalf("期望只保留 %d 个角色，实际 %d 个：\n%s", maxReferenceCharacters, n, got)
	}
	if strings.Contains(got, "丙") {
		t.Fatalf("超出上限的角色不应出现：\n%s", got)
	}
}

// ---------- V2.2 · 圣遗物套装效果（2026-09-15 新增） ----------

// 命中套装时，2/4 件效果都要进资料块，且照样标注「唯一依据」
func TestBuildAssetSummaryRelicSet(t *testing.T) {
	sets := []RelicSetRef{
		{
			Name: "绝缘之旗印",
			Effects: []RelicSetEffectRef{
				{Piece: 2, Effect: "元素充能效率提高20%"},
				{Piece: 4, Effect: "基于元素充能效率的25%，提高元素爆发造成的伤害，最多提高75%"},
			},
		},
	}

	got := BuildAssetSummary(nil, sets)
	for _, want := range []string{
		"【游戏资料】",
		"唯一依据",
		"绝缘之旗印",
		"2 件套 元素充能效率提高20%",
		"4 件套 基于元素充能效率的25%",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("资料文本缺少 %q：\n%s", want, got)
		}
	}
}

// 名字命中但库里没有效果 → 必须显式「暂无资料」，不能让它凭记忆描述（与角色同款保险）
func TestBuildAssetSummaryRelicSetNoData(t *testing.T) {
	got := BuildAssetSummary(nil, []RelicSetRef{{Name: "某新套装"}})

	if !strings.Contains(got, "某新套装") {
		t.Fatalf("应带上套装名：%s", got)
	}
	if !strings.Contains(got, "本服暂无该套装的效果资料") {
		t.Fatalf("必须显式标注暂无该套装资料：%s", got)
	}
}

// 命中套装过多时按上限截断
func TestBuildAssetSummaryCapsRelicSets(t *testing.T) {
	sets := []RelicSetRef{
		{Name: "甲套", Effects: []RelicSetEffectRef{{Piece: 2, Effect: "攻击力提高18%"}}},
		{Name: "乙套", Effects: []RelicSetEffectRef{{Piece: 2, Effect: "攻击力提高18%"}}},
		{Name: "丙套", Effects: []RelicSetEffectRef{{Piece: 2, Effect: "攻击力提高18%"}}},
	}

	got := BuildAssetSummary(nil, sets)

	if n := strings.Count(got, "\n- "); n != maxReferenceSets {
		t.Fatalf("期望只保留 %d 个套装，实际 %d 个：\n%s", maxReferenceSets, n, got)
	}
	if strings.Contains(got, "丙套") {
		t.Fatalf("超出上限的套装不应出现：\n%s", got)
	}
}

// 角色与套装同时命中：角色在前、套装在后，同处一个资料块
func TestBuildAssetSummaryCharsAndSets(t *testing.T) {
	refs := []CharacterStatRef{
		{Name: "胡桃", SubStats: []dto.StatWeightEntry{{Name: "暴击率", Weight: 10, Tier: "primary"}}},
	}
	sets := []RelicSetRef{
		{Name: "追忆之注连", Effects: []RelicSetEffectRef{{Piece: 4, Effect: "普通攻击、重击、下落攻击造成的伤害提高50%"}}},
	}

	got := BuildAssetSummary(refs, sets)

	if !strings.Contains(got, "胡桃") || !strings.Contains(got, "追忆之注连") {
		t.Fatalf("角色与套装都应出现：\n%s", got)
	}
	if strings.Index(got, "胡桃") > strings.Index(got, "追忆之注连") {
		t.Fatalf("角色资料应排在套装之前（更常被问）：\n%s", got)
	}
	if n := strings.Count(got, "\n- "); n != 2 {
		t.Fatalf("期望 2 行资料（1 角色 + 1 套装），实际 %d 行：\n%s", n, got)
	}
}

// 长效果文本不应把资料块撑爆：预算内只保留能放下的条目
func TestBuildAssetSummaryRespectsBudget(t *testing.T) {
	long := strings.Repeat("测", referenceTokenBudget)
	sets := []RelicSetRef{
		{Name: "甲套", Effects: []RelicSetEffectRef{{Piece: 4, Effect: long}}},
		{Name: "乙套", Effects: []RelicSetEffectRef{{Piece: 4, Effect: long}}},
	}

	got := BuildAssetSummary(nil, sets)

	// 第一条总会被收下（避免整块为空），第二条必须因预算被丢弃
	if !strings.Contains(got, "甲套") {
		t.Fatalf("第一条资料应保留：\n%s", got)
	}
	if strings.Contains(got, "乙套") {
		t.Fatalf("超出预算的条目不应出现：\n%s", got)
	}
}
