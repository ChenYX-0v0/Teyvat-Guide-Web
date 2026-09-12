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
