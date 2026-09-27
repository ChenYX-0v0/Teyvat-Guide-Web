package asset

// 端到端：种子 JSON → Import → 内存装载 → 算分（覆盖 P0-b 与 P0-c 的接缝，2026-09-27）
// 用 sqlite 临时库，与 internal/service/chat_session_test.go 同一套做法。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/database"
)

// 2026-09-27：评分权重就是 `subStats[].weight`，**归后台所有** ⇒ 导入必须"只补缺、不覆盖" ✗
// （生成器产出的是老的 10 分制优先级 ✗；直接覆盖会让**分数静默全错** ✗✗）
func TestStatWeightImportKeepsAdminWeights(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "stat_weight_score.db")
	if err := database.Init(config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("初始化 sqlite 失败：%v", err)
	}
	db := database.DB()
	// ⚠️ Import() 会遍历全部导入分支 ⇒ 这些表都得建出来（少一张就 "no such table" ✗）
	if err := db.AutoMigrate(
		&Character{}, &Weapon{}, &RelicSet{}, &RelicSetEffect{}, &TextMapEntry{},
		&PropMap{}, &FightProp{}, &RelicSlot{}, &Affix{}, &CharacterStatWeights{},
		&CharacterRelicWeight{}, &CharacterRelicWeightRule{}, &CharacterRelicWeightMP{}, &Missing{},
	); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	svc := New(db)

	// ① 模拟"后台编辑过 / 迁移后"的 payload：评分权重写在 subStats[].weight（0~100 ✓）+ 元素覆盖
	fromAdmin := StatRecommendation{
		SubStats: []StatWeightItem{
			{StatKey: "FIGHT_PROP_CRITICAL", Weight: 100, Tier: "primary"},
			// 迁移时补出来的维度（生成器那份里没有这条 ✓）
			{StatKey: "FIGHT_PROP_CRITICAL_HURT", Weight: 100, Tier: "primary"},
		},
		MainStats: map[string]string{"circlet": "暴击率"},
		ByElement: map[string]map[string]int{"冰": {"FIGHT_PROP_CRITICAL": 100}},
	}
	if err := svc.Import(ImportPayload{StatWeights: []StatWeightImport{{CharacterID: 10000007, Rec: fromAdmin}}}); err != nil {
		t.Fatalf("首次导入失败：%v", err)
	}

	// ② 再导入**生成器那份**（老 10 分制权重 ✗、且没有 byElement）⇒ 后台的权重/元素覆盖必须留住 ✓
	generated := StatRecommendation{
		SubStats: []StatWeightItem{
			{StatKey: "FIGHT_PROP_CRITICAL", Weight: 8, Tier: "secondary"},      // 覆盖应被忽略 ✗
			{StatKey: "FIGHT_PROP_ATTACK_PERCENT", Weight: 10, Tier: "primary"}, // 新词条，采纳 ✓
		},
		MainStats: map[string]string{"goblet": "攻击力"},
	}
	if err := svc.Import(ImportPayload{StatWeights: []StatWeightImport{{CharacterID: 10000007, Rec: generated}}}); err != nil {
		t.Fatalf("二次导入失败：%v", err)
	}

	var row CharacterStatWeights
	if err := db.First(&row, "character_id = ?", 10000007).Error; err != nil {
		t.Fatalf("读取 payload 失败：%v", err)
	}
	var got StatRecommendation
	if err := json.Unmarshal([]byte(row.Payload), &got); err != nil {
		t.Fatalf("解析 payload 失败：%v", err)
	}
	byKey := make(map[string]StatWeightItem, len(got.SubStats))
	for _, s := range got.SubStats {
		byKey[s.StatKey] = s
	}
	if w := byKey["FIGHT_PROP_CRITICAL"].Weight; w != 100 {
		t.Fatalf("后台的评分权重被导入覆盖了 ✗：暴击率 weight=%d（应为 100）", w)
	}
	if byKey["FIGHT_PROP_CRITICAL_HURT"].Weight != 100 {
		t.Fatal("后台补出来的维度被导入删掉了 ✗")
	}
	if byKey["FIGHT_PROP_ATTACK_PERCENT"].Weight != 10 {
		t.Fatal("导入件里的新词条应被采纳 ✓")
	}
	if got.ByElement["冰"] == nil {
		t.Fatal("旅行者的元素覆盖被导入冲掉了 ✗")
	}
	if got.MainStats["goblet"] != "攻击力" {
		t.Fatal("主词条应随导入更新 ✓")
	}
}

func TestRelicWeightsSeedImportAndScore(t *testing.T) {
	seed := filepath.Join("..", "..", "..", "cmd", "assetimport", "relic_weights_miao.json")
	raw, err := os.ReadFile(seed)
	if err != nil {
		t.Skipf("种子文件不存在，跳过：%v", err)
	}

	// ① 键名契约：种子必须能被 ImportPayload 直接吃下（否则管理接口会静默导入 0 条）
	var payload ImportPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("解析种子失败：%v", err)
	}
	if len(payload.RelicWeights) != 124 {
		t.Fatalf("权重点数应为 124（我们库里全部角色），实际 %d", len(payload.RelicWeights))
	}
	// 2026-09-27 起：3 条角色级（菲林斯专武 / 哥伦比娅专武 + 高命）+ 6 条通用级
	// （武器适配 4 把 / 西风 1 / 绝缘4 1）—— 通用级就是原先**硬编码在 Go** 的三类动态规则，
	// 落库后只改数据、不发版（所以这里也顺带守住"种子必须覆盖全部三类"的数据不变式）。
	if len(payload.RelicWeightRules) != 9 {
		t.Fatalf("覆盖规则应为 9 条（3 角色级 + 6 通用级），实际 %d", len(payload.RelicWeightRules))
	}
	var globalCount int
	for _, r := range payload.RelicWeightRules {
		if r.Effect == "" {
			t.Fatalf("规则缺 effect（应为 delta / delta_refine / raise_to / raise_to_max_of）：%+v", r)
		}
		if r.CharacterID != 0 {
			continue
		}
		globalCount++
		if r.Priority == 0 {
			t.Fatalf("通用规则必须带 priority（顺序会影响结果）：%+v", r)
		}
		if r.CondType == "weapon_regex" {
			if _, ok := relicRuleRegex(r.CondValue); !ok {
				t.Fatalf("通用规则的正则无法编译：%s", r.CondValue)
			}
		}
	}
	if globalCount != 6 {
		t.Fatalf("通用规则应为 6 条（4 武器适配 + 西风 + 绝缘4），实际 %d", globalCount)
	}

	// ② 真实入库（sqlite 临时库）
	dsn := filepath.Join(t.TempDir(), "relic_import_test.db")
	if err := database.Init(config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("初始化 sqlite 失败：%v", err)
	}
	db := database.DB()
	if err := db.AutoMigrate(
		&Character{}, &Weapon{}, &RelicSet{}, &RelicSetEffect{}, &TextMapEntry{},
		&PropMap{}, &FightProp{}, &RelicSlot{}, &Affix{}, &CharacterStatWeights{},
		&CharacterRelicWeight{}, &CharacterRelicWeightRule{}, &CharacterRelicWeightMP{}, &Missing{},
	); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	svc := New(db)
	if err := svc.Import(ImportPayload{
		RelicWeights:     payload.RelicWeights,
		RelicWeightRules: payload.RelicWeightRules,
	}); err != nil {
		t.Fatalf("导入失败：%v", err)
	}

	// ③ 装载：菲林斯（10000120）权重与已核对过的真值一致
	w, ok := svc.RelicWeight(10000120)
	if !ok {
		t.Fatal("菲林斯权重未装载")
	}
	if w.ATK != 100 || w.Mastery != 50 || w.Recharge != 50 || w.Dmg != 0 {
		t.Fatalf("菲林斯权重与真值不符：atk=%d mastery=%d recharge=%d dmg=%d",
			w.ATK, w.Mastery, w.Recharge, w.Dmg)
	}

	// ④ 覆盖规则生效：专武「血染荒城」→ 充能 -5
	eff, tag := svc.effectiveRelicWeight(RelicScoreInput{CharacterID: 10000120, WeaponName: "血染荒城"})
	if eff.Recharge != 45 || tag != "专武" {
		t.Fatalf("菲林斯专武规则未生效：recharge=%d tag=%q", eff.Recharge, tag)
	}

	// ④-b 通用规则经 DB 装载同样生效（本次改造的回归点：它们以前是 Go 里的硬编码）
	//      绝缘 4 件 → 充能拉齐 atk/hp/def/mastery 最高者（菲林斯 atk=100 ⇒ 充能 100）
	insul, tagI := svc.effectiveRelicWeight(RelicScoreInput{
		CharacterID: 10000120, SetName: "绝缘之旗印", SetCount: 4,
	})
	if insul.Recharge != 100 || !strings.Contains(tagI, "绝缘4") {
		t.Fatalf("通用规则（绝缘4）未生效：recharge=%d tag=%q", insul.Recharge, tagI)
	}
	//      西风系列 → 暴击抬到 100（挑一个暴击权重 < 100 的角色才有观察效果）
	for id, w := range svc.relicWeights {
		if w.CPct <= 0 || w.CPct >= 100 {
			continue
		}
		xf, tagX := svc.effectiveRelicWeight(RelicScoreInput{CharacterID: id, WeaponName: "西风剑"})
		if xf.CPct != 100 || !strings.Contains(tagX, "西风") {
			t.Fatalf("通用规则（西风）未生效：角色 %d cpct=%d tag=%q", id, xf.CPct, tagX)
		}
		break
	}

	// ⑤ 幂等：重复导入不会堆出重复规则
	if err := svc.Import(ImportPayload{
		RelicWeights:     payload.RelicWeights,
		RelicWeightRules: payload.RelicWeightRules,
	}); err != nil {
		t.Fatalf("二次导入失败：%v", err)
	}
	if n := len(svc.RelicWeightRules(10000125)); n != 2 {
		t.Fatalf("哥伦比娅应有 2 条规则（幂等更新），实际 %d", n)
	}
	if _, ok := svc.RelicWeight(10000125); !ok {
		t.Fatal("哥伦比娅权重未装载")
	}

	// ⑥ 用导入进来的权重真算一件：菲林斯（atk/cpct/cdmg 100、精通 50、充能 50、增伤 0）
	//    副词条为 攻击力(小)/暴击率/暴击伤害/元素精通 —— 应能算分且增伤不计分
	res, ok := svc.ScoreRelic(RelicScoreInput{
		CharacterID:  10000120,
		Slot:         "EQUIP_BRACER",
		MainStatName: "生命值", MainStatValue: 4780,
		SubStats: []RelicStatInput{
			{Name: "暴击率", Value: 3.9, Count: 1},
			{Name: "暴击伤害", Value: 7.8, Count: 1},
			{Name: "百分比攻击力", Value: 5.8, Count: 1},
			{Name: "元素精通", Value: 21, Count: 1},
		},
	})
	if !ok {
		t.Fatal("菲林斯这件应能计分")
	}
	if res.Score <= 0 || res.Score > relicScoreScale {
		t.Fatalf("分数应在 (0, 66]，实际 %.4f", res.Score)
	}
	// 生命权重为 0 ⇒ 花的主词条（生命值）与生命副词条都不该出现在维度贡献里
	if _, has := res.Dims["hp"]; has {
		t.Fatal("菲林斯生命权重为 0，不应有生命维度贡献")
	}
	if _, has := res.Dims["hpPlus"]; has {
		t.Fatal("菲林斯生命权重为 0，不应有白值生命维度贡献")
	}
}
