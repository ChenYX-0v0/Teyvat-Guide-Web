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
		&CharacterRelicWeight{}, &CharacterRelicWeightRule{}, &Missing{},
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
