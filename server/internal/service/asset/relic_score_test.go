package asset

// 圣遗物评分单测（V3，2026-09-27）
// 系数/档位/分档/规则全部来自上游 miao-plugin（MIT）的实测值：
// 一旦上游口径变了或我们改错了常数，这些断言会立刻红灯 —— 见 ai/08 §7。

import (
	"math"
	"strings"
	"testing"
)

func almostEqual(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// 档位常数自证：全部由 basicNum = 3.885 推出
func TestRelicTierConstants(t *testing.T) {
	cases := []struct {
		dim  string
		want float64
	}{
		{"cpct", 3.885}, {"cdmg", 7.77}, {"hp", 5.8275}, {"atk", 5.8275}, {"def", 7.284375},
		{"atkPlus", 19.425}, {"hpPlus", 298.659375}, {"defPlus", 23.31},
		{"mastery", 23.31}, {"recharge", 6.475}, {"heal", 2.9884615384615385},
	}
	for _, c := range cases {
		if got := relicMaxTier[c.dim]; !almostEqual(got, c.want, 1e-9) {
			t.Fatalf("最大档 %s = %v，期望 %v", c.dim, got, c.want)
		}
	}
	// 均值 = 最大档 × 0.85 ⇒ 社区口径的 3.3 暴击 / 6.6 暴伤
	if got := relicAvgTier["cpct"]; !almostEqual(got, 3.30225, 1e-9) {
		t.Fatalf("暴击均值档 = %v，期望 3.30225", got)
	}
	if got := relicAvgTier["cdmg"]; !almostEqual(got, 6.6045, 1e-9) {
		t.Fatalf("暴伤均值档 = %v，期望 6.6045", got)
	}
	if got := relicBasicNum * 0.85; !almostEqual(got, 3.3022500000000003, 1e-9) {
		t.Fatalf("basicNum × 0.85 = %v", got)
	}
}

// 分档边界（上游 getMarkClass：取第一个 mark < 阈值的档）
func TestRelicClass(t *testing.T) {
	cases := []struct {
		mark float64
		want string
	}{
		{0, "D"}, {6.99, "D"}, {7, "C"}, {13.99, "C"}, {14, "B"}, {20.99, "B"},
		{21, "A"}, {28, "S"}, {35, "SS"}, {42, "SSS"}, {49, "ACE"}, {55.99, "ACE"},
		{56, "MAX"}, {66, "MAX"}, {80, "MAX"},
	}
	for _, c := range cases {
		if got := RelicClass(c.mark); got != c.want {
			t.Fatalf("RelicClass(%v) = %s，期望 %s", c.mark, got, c.want)
		}
	}
	// 套装评级 = 5 件平均分（用户截图三例：257 / 247 / 235）
	if cls, avg := RelicSetGrade(257, 5); cls != "ACE" || !almostEqual(avg, 51.4, 1e-9) {
		t.Fatalf("257/5 应为 ACE(51.4)，实际 %s(%.2f)", cls, avg)
	}
	if cls, _ := RelicSetGrade(247, 5); cls != "ACE" {
		t.Fatalf("247/5 应为 ACE，实际 %s", cls)
	}
	if cls, avg := RelicSetGrade(235, 5); cls != "SSS" || !almostEqual(avg, 47, 1e-9) {
		t.Fatalf("235/5 应为 SSS(47)，实际 %s(%.2f)", cls, avg)
	}
}

// 白值折算：小攻击 1 档的等效权重 = weight × 19.425/5.8275/(230+520)×100
// 百分比维度不受折算影响（fixWeight = weight）
func TestRelicBaseNormalization(t *testing.T) {
	w := CharacterRelicWeight{ATK: 75}
	attrs := relicAttrs(w, map[string]float64{"hp": 14000, "atk": 230, "def": 700})

	want := 75 * 19.425 / 5.8275 / 750 * 100 // ≈ 0.4444 × 75 = 33.33
	if got := attrs["atkPlus"].fixWeight; !almostEqual(got, want, 1e-9) {
		t.Fatalf("小攻击 fixWeight = %v，期望 %v", got, want)
	}
	if got := attrs["atk"].fixWeight; !almostEqual(got, 75, 1e-9) {
		t.Fatalf("百分比攻击 fixWeight 应等于权重 75，实际 %v", got)
	}
	// mark 同理：攻% = weight / 5.8275；小攻击 = weight / 5.8275 / 750 × 100
	if got := attrs["atk"].mark; !almostEqual(got, 75/5.8275, 1e-9) {
		t.Fatalf("百分比攻击 mark = %v", got)
	}
	if got := attrs["atkPlus"].mark; !almostEqual(got, 75/5.8275/750*100, 1e-9) {
		t.Fatalf("小攻击 mark = %v", got)
	}
	// 权 0 的维度不进 attrs（上游行为：不参与计分）
	if _, ok := attrs["hp"]; ok {
		t.Fatal("HP 权重为 0 时不应进入 attrs")
	}
}

// 角色级覆盖规则：用已核对过的真值（菲林斯 / 哥伦比娅）
func TestRelicWeightRulesFromSeed(t *testing.T) {
	// 菲林斯：专武「血染荒城」→ 充能 -5
	feilinsi := &Service{
		relicWeights: map[int64]CharacterRelicWeight{
			10000120: {CharacterID: 10000120, ATK: 100, CPct: 100, CDmg: 100, Mastery: 50, Recharge: 50},
		},
		relicRules: map[int64][]CharacterRelicWeightRule{
			10000120: {{
				CharacterID: 10000120, CondType: "weapon", CondOp: "=", CondValue: "血染荒城",
				StatKey: "recharge", Delta: -5, Note: "专武",
			}},
		},
	}
	w, tag := feilinsi.effectiveRelicWeight(RelicScoreInput{CharacterID: 10000120, WeaponName: "血染荒城"})
	if w.Recharge != 45 {
		t.Fatalf("菲林斯 + 专武：充能权重应为 45，实际 %d", w.Recharge)
	}
	if tag != "专武" {
		t.Fatalf("应打上「专武」标签，实际 %q", tag)
	}
	// 未命中武器 → 保持默认
	if w2, tag2 := feilinsi.effectiveRelicWeight(RelicScoreInput{CharacterID: 10000120, WeaponName: "西风长枪"}); w2.Recharge != 50 || tag2 == "专武" {
		t.Fatalf("非专武不应改充能：recharge=%d tag=%q", w2.Recharge, tag2)
	}

	// 哥伦比娅：专武 -15 且 4 命 -15 ⇒ 100 → 70（可叠加）
	columbina := &Service{
		relicWeights: map[int64]CharacterRelicWeight{
			10000125: {CharacterID: 10000125, HP: 100, CPct: 100, CDmg: 100, Mastery: 75, Recharge: 100},
		},
		relicRules: map[int64][]CharacterRelicWeightRule{
			10000125: {
				{CondType: "weapon", CondOp: "=", CondValue: "帷间夜曲", StatKey: "recharge", Delta: -15, Note: "专武"},
				{CondType: "cons", CondOp: ">=", CondValue: "4", StatKey: "recharge", Delta: -15, Note: "高命"},
			},
		},
	}
	w3, tag3 := columbina.effectiveRelicWeight(RelicScoreInput{CharacterID: 10000125, WeaponName: "帷间夜曲", Constellation: 4})
	if w3.Recharge != 70 {
		t.Fatalf("哥伦比娅 专武+4命：充能权重应为 70，实际 %d", w3.Recharge)
	}
	if !strings.Contains(tag3, "专武") || !strings.Contains(tag3, "高命") {
		t.Fatalf("标签应含 专武|高命，实际 %q", tag3)
	}
	// 只有专武（0 命）→ 85
	if w4, _ := columbina.effectiveRelicWeight(RelicScoreInput{CharacterID: 10000125, WeaponName: "帷间夜曲"}); w4.Recharge != 85 {
		t.Fatalf("哥伦比娅 + 专武（0 命）应为 85，实际 %d", w4.Recharge)
	}
	// 只有 4 命 → 85
	if w5, _ := columbina.effectiveRelicWeight(RelicScoreInput{CharacterID: 10000125, Constellation: 4}); w5.Recharge != 85 {
		t.Fatalf("哥伦比娅 4 命（无专武）应为 85，实际 %d", w5.Recharge)
	}
}

// 上游动态规则（武器适配 / 西风系列 / 绝缘4）—— 2026-09-27 起**全部走数据**：
// 它们是 character_id = 0 的通用规则行，Go 里不再有硬编码表。
// 本用例用与线上种子一致的行构造 Service，断言与数据化之前**逐条等价**。
func relicGlobalRules() map[int64][]CharacterRelicWeightRule {
	return map[int64][]CharacterRelicWeightRule{
		0: {
			{
				CondType: "weapon", CondOp: "=", CondValue: "磐岩结绿", StatKey: "hp",
				Effect: RelicEffectDeltaRefine, ValueMin: 15, ValueMax: 30,
				RequireStat: "atk", RequireMin: 1, Priority: 20, Note: "磐岩结绿",
			},
			{
				CondType: "weapon_regex", CondOp: "=", CondValue: `^西风(长枪|大剑|剑|猎弓|秘典)$`,
				StatKey: "cpct", Effect: RelicEffectRaiseTo, Value: 100, Priority: 30, Note: "西风",
			},
			{
				CondType: "set", CondOp: ">=", CondValue: "绝缘之旗印", CondNum: 4,
				StatKey: "recharge", Effect: RelicEffectRaiseToMaxOf, Dims: "atk,hp,def,mastery",
				Priority: 40, Note: "绝缘4",
			},
		},
	}
}

func TestRelicDynamicRules(t *testing.T) {
	// 西风 → 暴击抬到 100（只升不降）
	s := &Service{
		relicWeights: map[int64]CharacterRelicWeight{
			1: {CharacterID: 1, ATK: 75, CPct: 60, CDmg: 100, Dmg: 100},
		},
		relicRules: relicGlobalRules(),
	}
	w, tag := s.effectiveRelicWeight(RelicScoreInput{CharacterID: 1, WeaponName: "西风剑"})
	if w.CPct != 100 || !strings.Contains(tag, "西风") {
		t.Fatalf("西风剑应把暴击拉到 100 并打标签，实际 cpct=%d tag=%q", w.CPct, tag)
	}
	// 暴击本就 100 → 不改动、也不打标签（与数据化之前一致）
	s100 := &Service{
		relicWeights: map[int64]CharacterRelicWeight{9: {CharacterID: 9, ATK: 75, CPct: 100}},
		relicRules:   relicGlobalRules(),
	}
	if _, tg := s100.effectiveRelicWeight(RelicScoreInput{CharacterID: 9, WeaponName: "西风剑"}); strings.Contains(tg, "西风") {
		t.Fatalf("暴击已 100 时不应重复打「西风」标签，实际 %q", tg)
	}
	// 非西风武器 → 不命中
	if w2, _ := s.effectiveRelicWeight(RelicScoreInput{CharacterID: 1, WeaponName: "磐岩结绿"}); w2.CPct != 60 {
		t.Fatalf("非西风武器不应改暴击，实际 %d", w2.CPct)
	}

	// 武器适配：磐岩结绿 精1 +15 / 精5 +30（线性），且只对攻击型角色生效
	s2 := &Service{
		relicWeights: map[int64]CharacterRelicWeight{
			2: {CharacterID: 2, ATK: 75, HP: 20},
			8: {CharacterID: 8, ATK: 0, HP: 20}, // 非攻击型（如精通/治疗型）
		},
		relicRules: relicGlobalRules(),
	}
	if w3, _ := s2.effectiveRelicWeight(RelicScoreInput{CharacterID: 2, WeaponName: "磐岩结绿", WeaponAffix: 1}); w3.HP != 35 {
		t.Fatalf("磐岩结绿精1 应为 20+15=35，实际 %d", w3.HP)
	}
	if w4, _ := s2.effectiveRelicWeight(RelicScoreInput{CharacterID: 2, WeaponName: "磐岩结绿", WeaponAffix: 5}); w4.HP != 50 {
		t.Fatalf("磐岩结绿精5 应为 20+30=50，实际 %d", w4.HP)
	}
	if w5, _ := s2.effectiveRelicWeight(RelicScoreInput{CharacterID: 8, WeaponName: "磐岩结绿", WeaponAffix: 5}); w5.HP != 20 {
		t.Fatalf("非攻击型角色不应吃武器适配（requireStat=atk），实际 hp=%d", w5.HP)
	}

	// 绝缘4 → 充能拉到 atk/hp/def/mastery 最高者（75）
	s3 := &Service{
		relicWeights: map[int64]CharacterRelicWeight{
			3: {CharacterID: 3, ATK: 75, Recharge: 30, CPct: 100, CDmg: 100, Dmg: 100},
		},
		relicRules: relicGlobalRules(),
	}
	w6, tag6 := s3.effectiveRelicWeight(RelicScoreInput{CharacterID: 3, SetName: "绝缘之旗印", SetCount: 4})
	if w6.Recharge != 75 || !strings.Contains(tag6, "绝缘4") {
		t.Fatalf("绝缘4 应把充能拉到 75，实际 recharge=%d tag=%q", w6.Recharge, tag6)
	}
	// 只有 2 件 → 不触发
	if w7, _ := s3.effectiveRelicWeight(RelicScoreInput{CharacterID: 3, SetName: "绝缘之旗印", SetCount: 2}); w7.Recharge != 30 {
		t.Fatalf("绝缘 2 件不应触发，实际 %d", w7.Recharge)
	}
}

// 顺序敏感：武器适配（priority 20）先加攻击，绝缘4（priority 40）再让充能拉齐 —— 与上游一致。
func TestRelicRulePriorityOrder(t *testing.T) {
	s := &Service{
		relicWeights: map[int64]CharacterRelicWeight{
			4: {CharacterID: 4, ATK: 60, Recharge: 10},
		},
		relicRules: map[int64][]CharacterRelicWeightRule{
			0: {
				{
					CondType: "set", CondOp: ">=", CondValue: "绝缘之旗印", CondNum: 4,
					StatKey: "recharge", Effect: RelicEffectRaiseToMaxOf, Dims: "atk,hp,def,mastery",
					Priority: 40, Note: "绝缘4",
				},
				{
					CondType: "weapon", CondOp: "=", CondValue: "猎人之径", StatKey: "atk",
					Effect: RelicEffectDelta, Delta: 30, Priority: 20, Note: "猎人之径",
				},
			},
		},
	}
	w, tag := s.effectiveRelicWeight(RelicScoreInput{CharacterID: 4, SetName: "绝缘之旗印", SetCount: 4, WeaponName: "猎人之径"})
	if w.ATK != 90 {
		t.Fatalf("武器适配应先加攻击到 90，实际 %d", w.ATK)
	}
	if w.Recharge != 90 {
		t.Fatalf("绝缘4 应看到加完后的攻击 90 并拉齐，实际 %d（说明 Priority 没生效）", w.Recharge)
	}
	if !strings.Contains(tag, "绝缘4") {
		t.Fatalf("标签应含 绝缘4，实际 %q", tag)
	}
}

// 算分主流程：能算、结果在界内、副词条更好则分更高
// 2026-09-27 用户决定：条件规则**不参与算分** ✗ —— ScoreRelic 必须忽略 character_relic_weight_rules。
// 反例（旧行为）：西风系列会把暴击权重抬到 100，从而直接改变单件分；现在不该有任何影响。
func TestScoreRelicIgnoresConditionRules(t *testing.T) {
	relic := RelicScoreInput{
		CharacterID:   12345,
		Slot:          "EQUIP_BRACER",
		MainStatName:  "生命值",
		MainStatValue: 4780,
		WeaponName:    "西风剑", // 命中通用规则（旧行为：暴击率 → 100）
		SubStats: []RelicStatInput{
			{Name: "暴击率", Value: 3.9, Count: 1},
			{Name: "暴击伤害", Value: 7.8, Count: 1},
			{Name: "元素充能效率", Value: 5.2, Count: 1},
		},
	}
	weights := map[int64]CharacterRelicWeight{
		12345: {CharacterID: 12345, ATK: 75, CPct: 60, CDmg: 100, Recharge: 50},
	}
	noRule := &Service{relicWeights: weights}
	withRule := &Service{relicWeights: weights, relicRules: relicGlobalRules()}

	a, okA := noRule.ScoreRelic(relic)
	b, okB := withRule.ScoreRelic(relic)
	if !okA || !okB {
		t.Fatal("两件都应能计分")
	}
	if !almostEqual(a.Score, b.Score, 1e-9) {
		t.Fatalf("规则不能再影响算分：无规则=%.6f 有规则=%.6f", a.Score, b.Score)
	}
	// 规则数据与函数**保留**（便于参考/回滚 ✓）—— 但它们不在算分链路上
	if w, tag := withRule.effectiveRelicWeight(relic); w.CPct != 100 || tag == "" {
		t.Fatalf("effectiveRelicWeight 应仍能算出规则效果（保留 ✓），实际 cpct=%d tag=%q", w.CPct, tag)
	}
}

func TestRelicScoreBasicAndMonotonic(t *testing.T) {
	s := &Service{relicWeights: map[int64]CharacterRelicWeight{
		10000046: {CharacterID: 10000046, HP: 80, ATK: 50, CPct: 100, CDmg: 100, Mastery: 75, Dmg: 100},
	}}
	flower := RelicScoreInput{
		CharacterID:  10000046,
		Slot:         "EQUIP_BRACER",
		MainStatName: "生命值", MainStatValue: 4780,
		SubStats: []RelicStatInput{
			{Name: "暴击率", Value: 3.9, Count: 1},
			{Name: "暴击伤害", Value: 7.8, Count: 1},
			{Name: "百分比攻击力", Value: 5.8, Count: 1},
			{Name: "元素精通", Value: 21, Count: 1},
		},
	}
	res, ok := s.ScoreRelic(flower)
	if !ok {
		t.Fatal("应能计分")
	}
	if res.Score <= 0 || res.Score > relicScoreScale+1e-6 {
		t.Fatalf("单件分应在 (0, 66]，实际 %.4f", res.Score)
	}
	if res.Percent <= 0 || res.Percent > 100 {
		t.Fatalf("百分位应在 (0, 100]，实际 %.4f", res.Percent)
	}
	if res.Class != RelicClass(res.Score) || res.Class == "" {
		t.Fatalf("档位异常：%q", res.Class)
	}
	// 双暴分 = 3.9×2 + 7.8
	if !almostEqual(res.CritScore, 15.6, 1e-9) {
		t.Fatalf("双暴分应为 15.6，实际 %.4f", res.CritScore)
	}
	// 总词条数 ≈ 3.9/3.30225 + 7.8/6.6045 + 5.8/4.953375 + 21/19.8135
	wantWords := 3.9/3.30225 + 7.8/6.6045 + 5.8/4.953375 + 21/19.8135
	if !almostEqual(res.WordCount, wantWords, 1e-9) {
		t.Fatalf("总词条数 %.4f，期望 %.4f", res.WordCount, wantWords)
	}
	// 有效词条 = 加权（cpct 100% / cdmg 100% / atk 50% / mastery 75%）
	wantEff := 3.9/3.30225*1.0 + 7.8/6.6045*1.0 + 5.8/4.953375*0.5 + 21/19.8135*0.75
	if !almostEqual(res.EffCount, wantEff, 1e-9) {
		t.Fatalf("有效词条 %.4f，期望 %.4f", res.EffCount, wantEff)
	}

	// 单调性：暴击 3.9 → 7.8，分数必须上升
	better := flower
	better.SubStats = append([]RelicStatInput(nil), flower.SubStats...)
	better.SubStats[0].Value = 7.8
	res2, _ := s.ScoreRelic(better)
	if res2.Score <= res.Score {
		t.Fatalf("副词条更好应得更高分：%.4f → %.4f", res.Score, res2.Score)
	}
}

// 满配件的理论上限就是 66 分（首条按 6 次强化计）
func TestRelicScoreIdealPieceEqualsScale(t *testing.T) {
	s := &Service{relicWeights: map[int64]CharacterRelicWeight{
		10000046: {CharacterID: 10000046, HP: 80, ATK: 50, CPct: 100, CDmg: 100, Mastery: 75, Dmg: 100},
	}}
	// 该角色副词条最优 4 条 = 暴击 100 / 暴伤 100 / 生命 80 / 精通 75
	// 满配：首条 6 档、其余各 1 档 ⇒ raw == posMax ⇒ 66 分
	ideal := RelicScoreInput{
		CharacterID:  10000046,
		Slot:         "EQUIP_BRACER",
		MainStatName: "生命值", MainStatValue: 4780,
		SubStats: []RelicStatInput{
			{Name: "暴击率", Value: relicMaxTier["cpct"] * 6},
			{Name: "暴击伤害", Value: relicMaxTier["cdmg"]},
			{Name: "百分比生命值", Value: relicMaxTier["hp"]},
			{Name: "元素精通", Value: relicMaxTier["mastery"]},
		},
	}
	res, ok := s.ScoreRelic(ideal)
	if !ok {
		t.Fatal("应能计分")
	}
	if !almostEqual(res.Score, relicScoreScale, 1e-6) {
		t.Fatalf("满配件应为 66 分，实际 %.6f（percent=%.2f）", res.Score, res.Percent)
	}
}

// 无法识别的部位 → 不计分（而不是给个假 0 分）
func TestRelicScoreUnknownSlot(t *testing.T) {
	s := &Service{}
	if _, ok := s.ScoreRelic(RelicScoreInput{Slot: "EQUIP_UNKNOWN"}); ok {
		t.Fatal("未知部位不应计分")
	}
}

// dto 字符串反解（translate 层已把数值格式化成 "19.4%" / "生命值 +4780"）
func TestRelicParseHelpers(t *testing.T) {
	if got := ParseRelicStatValue("19.4%"); !almostEqual(got, 19.4, 1e-9) {
		t.Fatalf("ParseRelicStatValue = %v", got)
	}
	if got := ParseRelicStatValue("4780"); !almostEqual(got, 4780, 1e-9) {
		t.Fatalf("ParseRelicStatValue = %v", got)
	}
	if got := ParseRelicStatValue(""); got != 0 {
		t.Fatalf("空串应为 0，实际 %v", got)
	}
	name, val := ParseRelicMainStat("生命值 +4780")
	if name != "生命值" || !almostEqual(val, 4780, 1e-9) {
		t.Fatalf("ParseRelicMainStat = (%q, %v)", name, val)
	}
	name2, val2 := ParseRelicMainStat("百分比攻击力 +46.6%")
	if name2 != "百分比攻击力" || !almostEqual(val2, 46.6, 1e-9) {
		t.Fatalf("ParseRelicMainStat = (%q, %v)", name2, val2)
	}
}
