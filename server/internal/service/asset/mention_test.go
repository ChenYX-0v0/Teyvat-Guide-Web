package asset

import "testing"

// V2 结构化查表靠名字命中；这里覆盖「长名优先」与「无命中」两个关键行为
func TestCharactersMentioned(t *testing.T) {
	s := &Service{
		characters: map[int64]Character{
			10000002: {ID: 10000002, Name: "神里绫华"},
			10000046: {ID: 10000046, Name: "胡桃"},
			10000052: {ID: 10000052, Name: "雷电将军"},
		},
	}

	got := s.CharactersMentioned("雷电将军和胡桃的圣遗物怎么选？")
	if len(got) != 2 {
		t.Fatalf("期望命中 2 个角色，实际 %d 个：%+v", len(got), got)
	}
	if got[0].Name != "雷电将军" {
		t.Fatalf("长名必须优先（否则「雷电将军」会被更短的名字抢先）：%+v", got)
	}
	if got[1].Name != "胡桃" {
		t.Fatalf("命中顺序不符：%+v", got)
	}

	// map 遍历本身无序，结果必须稳定可复现
	again := s.CharactersMentioned("雷电将军和胡桃的圣遗物怎么选？")
	for i := range got {
		if got[i].Name != again[i].Name {
			t.Fatalf("同一提问两次结果不一致：%+v vs %+v", got, again)
		}
	}
}

func TestCharactersMentionedNoMatch(t *testing.T) {
	s := &Service{
		characters: map[int64]Character{10000046: {ID: 10000046, Name: "胡桃"}},
	}
	if got := s.CharactersMentioned("今天天气怎么样"); len(got) != 0 {
		t.Fatalf("不该命中任何角色：%+v", got)
	}
	if got := s.CharactersMentioned("   "); len(got) != 0 {
		t.Fatalf("空提问应返回空：%+v", got)
	}
}

// 形态别名（2026-10-14 用户点名）：玩家只说「火主 / 冰主」✗，这两个词里**不含"旅行者"三字** ✗
// ⇒ 纯按名字匹配会完全命中不到、攻略卡根本不注入 ✓（火主 / 冰主是用的人最多的两个 ✓）
func TestCharactersMentionedTravelerAliases(t *testing.T) {
	s := &Service{
		characters: map[int64]Character{
			10000005: {ID: 10000005, Name: "旅行者"},
			10000046: {ID: 10000046, Name: "胡桃"},
		},
	}
	for _, q := range []string{"火主怎么配队？", "冰主的毕业面板是多少", "草主需要多少充能", "旅行者的圣遗物怎么选"} {
		got := s.CharactersMentioned(q)
		found := false
		for _, c := range got {
			if c.Name == "旅行者" {
				found = true
			}
		}
		if !found {
			t.Fatalf("提问 %q 应命中旅行者，实际：%+v", q, got)
		}
		if len(got) != 1 {
			t.Fatalf("提问 %q 只该命中旅行者一次（别重复追加 ✗），实际：%+v", q, got)
		}
	}

	// 别名没出现时**不许**乱补旅行者 ✗（否则 124 位角色的问题都会多带一张旅行者卡 ✓）
	if got := s.CharactersMentioned("胡桃怎么练？"); len(got) != 1 || got[0].Name != "胡桃" {
		t.Fatalf("没有形态别名时不该补旅行者，实际：%+v", got)
	}
}

// 角色别名（「雷神」→「雷电将军」「帝君」→「钟离」✓，2026-10-14 加 ✓）
//
// 为什么必须有 ✗：玩家几乎不说全名 ✓ —— 只说「雷神」「帝君」「万叶」✓；
//
//	名字匹配是**精确匹配** ✗ ⇒ 这些问法一张卡（攻略/机制/词条）都注入不了 ✓ ⇒ 只能靠模型记忆泛谈 ✓。
func TestCharactersMentionedAliases(t *testing.T) {
	s := &Service{
		characters: map[int64]Character{
			10000052: {ID: 10000052, Name: "雷电将军"},
			10000030: {ID: 10000030, Name: "钟离"},
			10000047: {ID: 10000047, Name: "枫原万叶"},
			10000046: {ID: 10000046, Name: "胡桃"},
		},
		aliases: map[string]string{
			"雷神": "雷电将军", "岩王帝君": "钟离", "帝君": "钟离", "万叶": "枫原万叶",
		},
	}
	for _, tc := range []struct{ q, want string }{
		{"雷神怎么配队？", "雷电将军"},
		{"帝君的圣遗物怎么选", "钟离"},
		{"岩王帝君带什么武器", "钟离"},
		{"万叶能进胡桃队吗", "枫原万叶"},
	} {
		got := s.CharactersMentioned(tc.q)
		found := false
		for _, c := range got {
			if c.Name == tc.want {
				found = true
			}
		}
		if !found {
			t.Fatalf("提问 %q 应命中所属角色 %s，实际：%+v", tc.q, tc.want, got)
		}
	}

	// 同一角色被多个别名同时命中 ⇒ **只算一次** ✓（否则词条位/卡位数会被吃满 ✗）
	got := s.CharactersMentioned("岩王帝君和帝君是同一个吧")
	n := 0
	for _, c := range got {
		if c.Name == "钟离" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("别名命中必须去重（钟离只该出现一次 ✓），实际 %d 次：%+v", n, got)
	}

	// 别名表为空 / 没提别名 ⇒ 行为与改动前完全一致 ✓
	if got := s.CharactersMentioned("胡桃怎么练？"); len(got) != 1 || got[0].Name != "胡桃" {
		t.Fatalf("没有别名时不该多命中，实际：%+v", got)
	}
	empty := &Service{characters: map[int64]Character{10000046: {ID: 10000046, Name: "胡桃"}}}
	if got := empty.CharactersMentioned("雷神怎么配队？"); len(got) != 0 {
		t.Fatalf("别名表为空时不该命中任何角色（老库降级 ✓），实际：%+v", got)
	}
}

// fuzzyService 造一组用于容错测试的角色（含 3 字名与 2 字名各一 ✓）
func fuzzyService() *Service {
	return &Service{
		characters: map[int64]Character{
			10000120: {ID: 10000120, Name: "伊涅芙"},
			10000087: {ID: 10000087, Name: "那维莱特"},
			10000046: {ID: 10000046, Name: "胡桃"},
		},
	}
}

func namesOf(list []Character) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.Name)
	}
	return out
}

// 错别字容错（2026-10-14 用户报障 ✓）：问「伊涅**夫**怎么配队」✗ 也必须命中「伊涅**芙**」✓
// —— 不命中就整条链断掉（机制卡 / 攻略卡 / 词条全没有 ✓，模型只能老实说不知道 ✓）。
func TestCharactersMentionedFuzzyTypo(t *testing.T) {
	s := fuzzyService()
	got := s.CharactersMentioned("伊涅夫怎么配队")
	if len(got) != 1 || got[0].Name != "伊涅芙" {
		t.Fatalf("错别字「伊涅夫」应命中「伊涅芙」，实际：%+v", namesOf(got))
	}
}

// ① 写对的名字**行为一字不变** ✓（这条最重要 ✓：不能因为加了容错就把正常路径改了 ✓）
// ② 连跑多次结果稳定 ✓
func TestCharactersMentionedFuzzyDoesNotAffectExact(t *testing.T) {
	s := fuzzyService()
	for i := 0; i < 20; i++ {
		got := s.CharactersMentioned("伊涅芙怎么配队")
		if len(got) != 1 || got[0].Name != "伊涅芙" {
			t.Fatalf("正确名字只该命中一个，实际：%+v", namesOf(got))
		}
		if other := s.CharactersMentioned("胡桃怎么练"); len(other) != 1 || other[0].Name != "胡桃" {
			t.Fatalf("短名角色不受影响，实际：%+v", namesOf(other))
		}
	}
}

// 不许乱认 ✗ 之一：**两字名不启用容错** ✓（差一个字就是另一个名字了 ✓）
func TestCharactersMentionedFuzzySkipsShortNames(t *testing.T) {
	s := fuzzyService()
	if got := s.CharactersMentioned("胡逃怎么练"); len(got) != 0 {
		t.Fatalf("两字名不该走容错，实际：%+v", namesOf(got))
	}
}

// 不许乱认 ✗ 之二：**虚词不算错别字** ✓（句式的"的"不能被当成名字的错字 ✓）
func TestCharactersMentionedFuzzySkipsStopRunes(t *testing.T) {
	s := fuzzyService()
	if got := s.CharactersMentioned("那维莱的圣遗物怎么选"); len(got) != 0 {
		t.Fatalf("虚词差异不该命中，实际：%+v", namesOf(got))
	}
}

// 一次提问**只补一个**角色 ✓（防连锁误匹配 ✓）；且精确命中与容错**可以并存** ✓
func TestCharactersMentionedFuzzyOnlyOnePerQuestion(t *testing.T) {
	s := fuzzyService()

	// 两个错别字 ⇒ 只认一个（按"长名优先"取那维莱特 ✓，结果确定 ✓）
	one := s.CharactersMentioned("伊涅夫和那维来特怎么配队")
	if len(one) != 1 || one[0].Name != "那维莱特" {
		t.Fatalf("两个错别字时只该补一个（长名优先），实际：%+v", namesOf(one))
	}

	// 精确命中 ＋ 一个错别字 ⇒ 两者都要给 ✓（"胡桃和伊涅夫"⇒ 伊涅芙 ＋ 胡桃 ✓）
	// 顺序照既有口径 **长名优先** ✓（伊涅芙 3 字 在前 ✓）
	both := s.CharactersMentioned("胡桃和伊涅夫怎么配队")
	if len(both) != 2 || both[0].Name != "伊涅芙" || both[1].Name != "胡桃" {
		t.Fatalf("精确 ＋ 容错应各命中一个（长名优先），实际：%+v", namesOf(both))
	}
}

// 同名两行（`characters` 表里「旅行者」= 10000005 与 10000007 ✓）⇒ 必须**确定性**取 ID 最大那个 ✓
//
// 2026-10-14 实测到的抖动 ✗：取"map 迭代先撞到的"那一行 ⇒ 有时落到 10000005（没有攻略卡 ✗）
// ⇒ 同一句话有时答得出来、有时答不出来 ✓。所以这里连跑 20 次验"不会抖" ✓。
func TestCharactersMentionedDuplicateNameDeterministic(t *testing.T) {
	s := &Service{
		characters: map[int64]Character{
			10000005: {ID: 10000005, Name: "旅行者"},
			10000007: {ID: 10000007, Name: "旅行者"},
			10000046: {ID: 10000046, Name: "胡桃"},
		},
	}
	for i := 0; i < 20; i++ {
		got := s.CharactersMentioned("旅行者怎么练？")
		if len(got) != 1 || got[0].ID != 10000007 {
			t.Fatalf("同名两行必须去重且保留最大 ID，实际：%+v", got)
		}
		alias := s.CharactersMentioned("火主怎么练？")
		if len(alias) != 1 || alias[0].ID != 10000007 {
			t.Fatalf("形态别名也必须落到最大 ID，实际：%+v", alias)
		}
	}
}
