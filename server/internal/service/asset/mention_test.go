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
