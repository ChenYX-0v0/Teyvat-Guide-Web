package asset

// 小助手口径的算分自证（公式表来自用户提供的截图，逐条落成用例）
//
// 口径（2026-09-27 用户决定）：评分 = 数值 × 系数 × 收益权重(/100)，**不做归一化**；
// 主词条只算暴击率/暴击伤害（各 +20）；只有 7 维（增伤/物伤/治疗不计分）。

import (
	"math"
	"testing"
)

// 温迪的小助手权重（来自种子：攻击 75 / 暴击 100 / 暴伤 100 / 精通 75 / 充能 55）
func wen() CharacterRelicWeightMP {
	return CharacterRelicWeightMP{
		CharacterID: 10000059, Element: "", Name: "温迪",
		ATK: 75, CPCT: 100, CDMG: 100, Mastery: 75, Recharge: 55,
	}
}

func almostEqMP(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestMPScoreCoefficients(t *testing.T) {
	// 花（无主词条加分）：暴击 3.9 / 暴伤 7.8 / 大攻击 5.8% / 精通 21
	in := RelicScoreInput{
		CharacterID: 10000059,
		Slot:        "生之花",
		SubStats: []RelicStatInput{
			{Name: "暴击率", Value: 3.9},
			{Name: "暴击伤害", Value: 7.8},
			{Name: "百分比攻击力", Value: 5.8},
			{Name: "元素精通", Value: 21},
		},
	}
	res, ok := mpscore(in, wen())
	if !ok {
		t.Fatal("应当算得出分")
	}
	// 逐项：3.9×2 + 7.8×2 + 5.8×1.33×0.75 + 21×0.33×0.75
	want := 3.9*2 + 7.8*2 + 5.8*1.33*0.75 + 21*0.33*0.75
	if !almostEqMP(res.Score, want) {
		t.Fatalf("单件分 = %.4f，期望 %.4f", res.Score, want)
	}
	// 双暴分 = 暴击×2 + 暴伤（只看副词条）
	if !almostEqMP(res.CritScore, 3.9*2+7.8) {
		t.Fatalf("双暴分 = %.4f，期望 %.4f", res.CritScore, 3.9*2+7.8)
	}
}

func TestMPScoreMainStatBonus(t *testing.T) {
	// 理之冠：暴击伤害主词条 → 只 +20（不含主词条数值折算 —— 与小助手口径一致）
	sub := []RelicStatInput{{Name: "暴击率", Value: 3.9}}
	crit := RelicScoreInput{Slot: "理之冠", MainStatName: "暴击伤害 +62.2%", SubStats: sub}
	res, _ := mpscore(crit, wen())
	if !almostEqMP(res.Score, 3.9*2+20) {
		t.Fatalf("暴伤冠 = %.4f，期望 %.4f（3.9×2 + 20）", res.Score, 3.9*2+20)
	}
	// 暴击率主词条同样 +20
	rate := RelicScoreInput{Slot: "理之冠", MainStatName: "暴击率 +31.1%", SubStats: sub}
	res2, _ := mpscore(rate, wen())
	if !almostEqMP(res2.Score, 3.9*2+20) {
		t.Fatalf("暴击冠 = %.4f，期望 %.4f", res2.Score, 3.9*2+20)
	}
	// 其它主词条不加分（用户决定：先不算）
	other := RelicScoreInput{Slot: "理之冠", MainStatName: "攻击力 +46.6%", SubStats: sub}
	res3, _ := mpscore(other, wen())
	if !almostEqMP(res3.Score, 3.9*2) {
		t.Fatalf("攻击冠不应加分，实际 %.4f", res3.Score)
	}
	// 花/羽的主词条永远不加分
	flower := RelicScoreInput{Slot: "生之花", MainStatName: "生命值 +4780", SubStats: sub}
	res4, _ := mpscore(flower, wen())
	if !almostEqMP(res4.Score, 3.9*2) {
		t.Fatalf("花的主词条不应加分，实际 %.4f", res4.Score)
	}
}

func TestMPScoreDimensions(t *testing.T) {
	// 权重为 0 的维度不计分（如温迪不记生命）
	in := RelicScoreInput{
		Slot: "生之花",
		SubStats: []RelicStatInput{
			{Name: "百分比生命值", Value: 5.8},
			{Name: "元素充能效率", Value: 5.5},
		},
	}
	res, _ := mpscore(in, wen())
	if _, has := res.Dims["hp"]; has {
		t.Fatal("温迪生命权重为 0，不应有生命维度贡献")
	}
	if !almostEqMP(res.Score, 5.5*1.1979*0.55) {
		t.Fatalf("充能项 = %.4f，期望 %.4f", res.Score, 5.5*1.1979*0.55)
	}
	// 小攻击（白值）走固定系数 0.398×0.5
	in2 := RelicScoreInput{Slot: "生之花", SubStats: []RelicStatInput{{Name: "攻击力", Value: 19}}}
	res2, _ := mpscore(in2, wen())
	if !almostEqMP(res2.Score, 19*0.398*0.5*0.75) {
		t.Fatalf("小攻击项 = %.4f，期望 %.4f", res2.Score, 19*0.398*0.5*0.75)
	}
}
