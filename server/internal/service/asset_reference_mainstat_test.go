package service

// 主词条"二选一"的渲染回归（2026-09-27）
//
// 背景（线上实测）：库里 mainStats 每个部位只写一个值时，模型把"推荐"读成"唯一正确"，
// 于是把旅行者身上完全合法的另一件主词条判成"错的"——用户反馈原话：
// 「先把理之冠换成暴击率主词条，这是唯一不对的一条」✗（角色同时吃双暴时两条冠都成立）。
// 现在库里可写 "暴击率/暴击伤害"，渲染必须展开成「暴击率 或 暴击伤害」，
// 让模型看得出这是二选一、都成立。

import (
	"strings"
	"testing"

	"teyvat-guide/server/internal/dto"
)

func TestRenderAssetRefMainStatAlternatives(t *testing.T) {
	ref := CharacterStatRef{
		Name:     "温迪",
		SubStats: []dto.StatWeightEntry{{Name: "攻击力", Weight: 10, Tier: "primary"}},
		MainStats: []StatSlotRef{
			{Slot: "时之沙", Stat: "攻击力"},
			{Slot: "空之杯", Stat: "风元素伤害加成"},
			{Slot: "理之冠", Stat: "暴击率/暴击伤害"},
		},
	}
	got := renderAssetRef(ref)
	if !strings.Contains(got, "理之冠 暴击率 或 暴击伤害") {
		t.Fatalf("二选一未展开为「或」：%s", got)
	}
	if strings.Contains(got, "/") {
		t.Fatalf("不应残留分隔符「/」：%s", got)
	}
	// 沙/杯同样适用（攻击力/元素充能效率）
	ref.MainStats[0].Stat = "攻击力/元素充能效率"
	if alt := renderAssetRef(ref); !strings.Contains(alt, "时之沙 攻击力 或 元素充能效率") {
		t.Fatalf("沙的二选一未展开：%s", alt)
	}
	// 单值不受影响
	ref.MainStats[2].Stat = "治疗加成"
	if single := renderAssetRef(ref); !strings.Contains(single, "理之冠 治疗加成") {
		t.Fatalf("单值渲染被破坏：%s", single)
	}
}
