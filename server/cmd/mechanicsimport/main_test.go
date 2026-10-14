package main

import (
	"strings"
	"testing"
)

// renderMoonEnablers 必须**按"管哪条反应"分组** ✓，且"只拉月兆等级"的那几位**单列** ✗
//
// 这两条都是被用户当场点出来的坑 ✓（ai/07 的 I 段 ✓）：
//   · 2026-10-14「为什么月结晶和月感电会出现奈芙尔，这怎么想的」✓ ⇒ 压成一行会给出**错误搭配** ✗
//   · 爱诺 / 叶洛亚 / 雅珂达的天赋原文只有"月兆上升一级"✓ ⇒ 它们**不能开门** ✗，混进开门人里同样是错 ✓
func TestRenderMoonEnablersGrouping(t *testing.T) {
	f := moonEnablersFile{Enablers: []moonEnabler{
		{Name: "伊涅芙", Talent: "月兆祝赐·象拟中继", Branches: []string{"月感电"}, Verified: true},
		{Name: "奈芙尔", Talent: "月兆祝赐·廊下暮影", Branches: []string{"月绽放"}, Level: true, Verified: true},
		{Name: "哥伦比娅", Talent: "月兆祝赐·借汝月光", Branches: []string{"月感电", "月绽放", "月结晶"}, Verified: true},
		{Name: "爱诺", Talent: "月兆祝赐·力极演算", Level: true, Verified: true},
		{Name: "没核过的角色", Talent: "月兆祝赐·待核", Branches: []string{"月感电"}, Verified: false},
	}}
	got := renderMoonEnablers(f)

	line := func(prefix string) string {
		for _, l := range strings.Split(got, "\n") {
			if strings.HasPrefix(l, prefix) {
				return l
			}
		}
		return ""
	}

	// ① 每条反应只列对应的人 ✓（奈芙尔**只能**出现在月绽放那行 ✗）
	if l := line("· **月感电** = "); !strings.Contains(l, "伊涅芙「象拟中继」") || strings.Contains(l, "奈芙尔") {
		t.Fatalf("月感电那行只该有 伊涅芙（＋哥伦比娅走三条全通 ✓），实际：%s", l)
	}
	if l := line("· **月绽放** = "); !strings.Contains(l, "奈芙尔「廊下暮影」") {
		t.Fatalf("月绽放那行该有奈芙尔，实际：%s", l)
	}
	if l := line("· **三条全通** = "); !strings.Contains(l, "哥伦比娅「借汝月光」") {
		t.Fatalf("哥伦比娅该落在三条全通，实际：%s", l)
	}

	// ② 「只拉月兆等级」单列 ✓ 且写明不能当开门人用 ✗
	if l := line("⚠️ **只拉「月兆等级 +1」"); !strings.Contains(l, "爱诺「力极演算」") || !strings.Contains(l, "别当开门人用") {
		t.Fatalf("爱诺该落在「只拉月兆等级」并写明别当开门人用，实际：%s", l)
	}

	// ③ 未核实的条目**不许**进正文 ✗
	if strings.Contains(got, "没核过的角色") {
		t.Fatalf("verified=false 的条目不该渲染，实际：%s", got)
	}

	// ④ 不封口 + 不许张冠李戴，两条硬约束必须在 ✓
	for _, must := range []string{"不封口", "张冠李戴", "奈芙尔只能开月绽放"} {
		if !strings.Contains(got, must) {
			t.Fatalf("渲染结果缺约束 %q，实际：%s", must, got)
		}
	}
}

// 空事实源 ⇒ 渲染出"只有约束、没有名单"的文本 ✓（不能 panic ✗，也不要吐出半截名单 ✗）
func TestRenderMoonEnablersEmpty(t *testing.T) {
	got := renderMoonEnablers(moonEnablersFile{})
	if got == "" {
		t.Fatal("空输入也该给出约束文本（不能是空串）")
	}
	if strings.Contains(got, "· **月") {
		t.Fatalf("没有条目时不该出现名单行，实际：%s", got)
	}
}
