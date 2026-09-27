package enka

// 圣遗物评分·翻译层冒烟（V3，2026-09-27）：
// 验证 scoreRelics 把 asset 层的算分结果正确落到 card / relic 字段上。
// 空 DB（asset.New(nil)）下权重走默认兜底，评分链路依然可用 —— 正好覆盖"库里还没导权重"的兼容路径。

import (
	"math"
	"testing"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/service/asset"
)

func TestScoreRelicsFillsCardAndRelicFields(t *testing.T) {
	tr := NewTranslator(asset.New(nil))

	card := &dto.CharacterCard{
		ID:            10000002, // 神里绫华（空 DB 下无角色映射，仅当主键用）
		Constellation: 0,
		Relics: []dto.RelicInfo{
			{Set: "绝缘之旗印", Pos: "EQUIP_BRACER", Slot: "生之花"},
			{Set: "绝缘之旗印", Pos: "EQUIP_DRESS", Slot: "理之冠", MainStat: "暴击率 +31.1%"},
		},
	}
	inputs := []asset.RelicScoreInput{
		{
			Slot:          "EQUIP_BRACER",
			MainStatName:  "生命值",
			MainStatValue: 4780,
			SubStats: []asset.RelicStatInput{
				{Name: "暴击率", Value: 3.9},
				{Name: "暴击伤害", Value: 7.8},
			},
		},
		{
			Slot:          "EQUIP_DRESS",
			MainStatName:  "暴击率",
			MainStatValue: 31.1,
			SubStats: []asset.RelicStatInput{
				{Name: "暴击伤害", Value: 14.0},
			},
		},
	}

	tr.scoreRelics(card, inputs)

	// 单件：分数、档位、双暴分都要落上
	r0, r1 := &card.Relics[0], &card.Relics[1]
	if r0.Score <= 0 || r0.Score > 66 || r0.ScoreCls == "" {
		t.Fatalf("花：分/档异常 score=%.3f cls=%s", r0.Score, r0.ScoreCls)
	}
	if r1.Score <= 0 || r1.ScoreCls == "" {
		t.Fatalf("冠：分/档异常 score=%.3f cls=%s", r1.Score, r1.ScoreCls)
	}
	// 双暴分：花 = 3.9×2 + 7.8 = 15.6；冠副词条 = 14.0
	if math.Abs(r0.CritScore-15.6) > 1e-9 || math.Abs(r1.CritScore-14.0) > 1e-9 {
		t.Fatalf("双暴分不符：花 %.3f / 冠 %.3f", r0.CritScore, r1.CritScore)
	}
	// 绝缘4 未满 4 件（这里只有 2 件）⇒ 不该打"绝缘4"标签
	if r0.WeightTag != "" || r1.WeightTag != "" {
		t.Fatalf("绝缘 2 件不应触发规则标签：%q / %q", r0.WeightTag, r1.WeightTag)
	}

	// 汇总：total = 两件之和；avg = total/2；档位与 avg 一致
	wantTotal := r0.Score + r1.Score
	if math.Abs(card.RelicScoreTotal-wantTotal) > 1e-9 {
		t.Fatalf("总分 %.3f，期望 %.3f", card.RelicScoreTotal, wantTotal)
	}
	if math.Abs(card.RelicScoreAvg-wantTotal/2) > 1e-9 {
		t.Fatalf("平均分 %.3f，期望 %.3f", card.RelicScoreAvg, wantTotal/2)
	}
	if card.RelicScoreClass != asset.RelicClass(card.RelicScoreAvg) {
		t.Fatalf("汇总档位 %q 与平均分不符", card.RelicScoreClass)
	}
	if card.RelicWordCount <= 0 || card.RelicCritScore <= 0 {
		t.Fatalf("词条数/双暴分汇总异常：%.3f / %.3f", card.RelicWordCount, card.RelicCritScore)
	}
}

// 输入与圣遗物数量不齐（防御：翻译层结构变化时宁可不算也不能错位）
func TestScoreRelicsSkipsOnMismatch(t *testing.T) {
	tr := NewTranslator(asset.New(nil))
	card := &dto.CharacterCard{
		ID:     1,
		Relics: []dto.RelicInfo{{Pos: "EQUIP_BRACER"}},
	}
	tr.scoreRelics(card, nil) // inputs 为空
	if card.RelicScoreTotal != 0 || card.RelicScoreClass != "" {
		t.Fatalf("inputs 为空不应产出评分：%+v", card)
	}
	if card.Relics[0].Score != 0 {
		t.Fatalf("单件分数不应被写入：%v", card.Relics[0].Score)
	}
}
