package asset

// 端到端：种子 JSON → Import → 内存装载 → 算分（覆盖 P0-b 与 P0-c 的接缝，2026-09-27）
// 用 sqlite 临时库，与 internal/service/chat_session_test.go 同一套做法。

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	if len(payload.RelicWeightRules) != 3 {
		t.Fatalf("覆盖规则应为 3 条，实际 %d", len(payload.RelicWeightRules))
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
