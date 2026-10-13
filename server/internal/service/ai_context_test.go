package service

import (
	"fmt"
	"strings"
	"testing"

	"teyvat-guide/server/internal/dto"
)

// characterFixture 造一张"满配"角色卡（天赋 + 武器 + 5 件圣遗物 + 评分），
// 形状对齐线上真实数据 —— 明细条目的大小主要由这套字段决定。
func characterFixture(name string) dto.CharacterCard {
	slots := []string{"生之花", "死之羽", "时之沙", "空之杯", "理之冠"}
	relics := make([]dto.RelicInfo, 0, len(slots))
	for _, slot := range slots {
		relics = append(relics, dto.RelicInfo{
			Name:     name + "的" + slot,
			Set:      "黄金剧团",
			Slot:     slot,
			Rarity:   5,
			Level:    20,
			MainStat: "暴击伤害",
		})
	}
	return dto.CharacterCard{
		Name:            name,
		Element:         "水",
		Rarity:          5,
		Level:           90,
		Constellation:   2,
		Talents:         &dto.TalentInfo{Normal: 4, Skill: 10, Burst: 10},
		Weapon:          &dto.WeaponInfo{Name: "静水流涌之辉", Rarity: 5, Level: 90, Affix: 1},
		Relics:          relics,
		RelicScoreTotal: 201.5,
		RelicScoreAvg:   40.3,
		RelicScoreClass: "ACE",
		RelicWordCount:  30.2,
		RelicCritScore:  116.5,
	}
}

func profileFixture(n int) *dto.PlayerProfile {
	names := []string{
		"沃雅妮莎", "奥黛塔", "爱可菲", "丝柯克", "尼可", "薇斯纳", "芙宁娜",
		"哥伦比娅", "兹白", "菲林斯", "伊涅芙", "杜林", "纳西妲", "钟离",
		"雷电将军", "枫原万叶",
	}
	chars := make([]dto.CharacterCard, 0, n)
	for i := range n {
		chars = append(chars, characterFixture(fmt.Sprintf("%s%02d", names[i%len(names)], i/len(names))))
	}
	return &dto.PlayerProfile{UID: "197794747", Nickname: "测试", Level: 60, Characters: chars}
}

// 回归（2026-10-12）：ctxTokenBudget=1500 时，展柜满 12 位只进得去 7 位，
// 模型据此回答「剩下 5 位派蒙看不到、可能没放进展示柜」✗（线上实测）。
// 预算已上调到 3000 —— 这条测试用来钉住"12 位必须全部进上下文"。
func TestBuildPlayerContextKeepsAllShowcaseCharacters(t *testing.T) {
	rc := BuildPlayerContext(profileFixture(12), "我这些角色都该怎么练")
	if rc == nil {
		t.Fatal("上下文不应为 nil")
	}
	chars, ok := rc.Player["characters"].([]map[string]any)
	if !ok {
		t.Fatalf("characters 字段类型不对：%T", rc.Player["characters"])
	}
	if len(chars) != 12 {
		t.Fatalf("展柜 12 位，实际只进了 %d 位（预算又不够了 ✗）", len(chars))
	}
	if rc.Summary != "" {
		t.Fatalf("12 位都能装下时不应有省略摘要，实际：%s", rc.Summary)
	}
	for i, c := range chars {
		if _, has := c["relicScore"]; !has {
			t.Fatalf("第 %d 位（%v）缺少明细评分 —— 明细展开规则失效", i+1, c["name"])
		}
	}
}

// 预算真不够时，摘要必须给出**名字**：只报"省略了 N 个"会让模型把
// "本次没带上"说成"数据没同步 / 没这个角色"✗（2026-10-12 线上实测）。
func TestBuildPlayerContextSummaryKeepsOmittedNames(t *testing.T) {
	const n = 60 // 远超预算，逼出省略分支
	p := profileFixture(n)
	rc := BuildPlayerContext(p, "我这些角色都该怎么练")
	if rc == nil {
		t.Fatal("上下文不应为 nil")
	}
	chars, _ := rc.Player["characters"].([]map[string]any)
	if len(chars) >= n {
		t.Skip("预算已足够装下全部角色，跳过省略分支的断言")
	}
	if !strings.Contains(rc.Summary, "未展开") {
		t.Fatalf("摘要应说明明细未展开，实际：%s", rc.Summary)
	}
	last := p.Characters[n-1].Name
	if !strings.Contains(rc.Summary, last) {
		t.Fatalf("摘要没给出被省略角色的名字（缺 %s），实际：%s", last, rc.Summary)
	}
}
