package service

import (
	"strings"
	"testing"

	"teyvat-guide/server/internal/model"
)

func fullGuideRow() model.CharacterGuide {
	return model.CharacterGuide{
		Name:       "测试角色",
		Role:       "定位文本",
		Talents:    "天赋文本",
		WeaponPick: "武器文本",
		RelicPick:  "圣遗物文本",
		TeamPick:   "配队文本",
		TeamFrom:   "page",
		Graduation: "毕业文本",
		Notes:      "注意文本",
	}
}

// 段顺序就是**优先级**：配队 > 毕业线 > 天赋顺序 > 武器 > 圣遗物 > 定位 > 注意事项。
// 按用户抱怨的三类（配队思路 / 毕业线 / 天赋顺序）排在最前 ✓
func TestRenderGuideSegmentOrder(t *testing.T) {
	got := renderGuide(fullGuideRow(), false)

	want := []string{
		"【角色攻略】测试角色",
		"· 配队思路：配队文本",
		"· 毕业线：毕业文本",
		"· 天赋顺序：天赋文本",
		"· 武器：武器文本",
		"· 圣遗物：圣遗物文本",
		"· 定位：定位文本",
		"· 注意事项：注意文本",
	}
	last := -1
	for _, w := range want {
		i := strings.Index(got, w)
		if i < 0 {
			t.Fatalf("渲染结果缺少 %q\n实际：%s", w, got)
		}
		if i < last {
			t.Fatalf("段顺序不对：%q 出现在更前的位置\n实际：%s", w, got)
		}
		last = i
	}
}

// 超上限时**从低优先级丢**：配队 / 毕业线 / 天赋必须留下，注意事项先被丢掉 ✓
func TestRenderGuideDropsLowPriorityFirst(t *testing.T) {
	row := fullGuideRow()
	row.Notes = strings.Repeat("注", 3000) // 单段就超上限
	got := renderGuide(row, false)

	if strings.Contains(got, "注意事项") {
		t.Fatalf("超限时应先丢掉最低优先级的「注意事项」，实际仍包含：%s", got[:60])
	}
	for _, keep := range []string{"· 配队思路：", "· 毕业线：", "· 天赋顺序："} {
		if !strings.Contains(got, keep) {
			t.Fatalf("超限时不该丢 %q（用户抱怨的就是这三类）\n实际：%s", keep, got)
		}
	}
}

// 配队来自反向索引时要如实标注来源（"据其他角色攻略页"是**事实**，不是免责声明 ✓）
func TestRenderGuideReverseTeamLabel(t *testing.T) {
	row := fullGuideRow()
	row.TeamFrom = "reverse"
	if got := renderGuide(row, false); !strings.Contains(got, "配队思路（据其他角色攻略页）：") {
		t.Fatalf("反向索引的配队应带来源说明，实际：%s", got)
	}
	row.TeamFrom = "page"
	if got := renderGuide(row, false); strings.Contains(got, "据其他角色攻略页") {
		t.Fatalf("本页原文的配队不该带反向说明，实际：%s", got)
	}
}

// 一条数据都没有时返回空串 —— 宁可**不注入**，也不要给模型一个光秃秃的「【角色攻略】某某」✗
func TestRenderGuideEmpty(t *testing.T) {
	if got := renderGuide(model.CharacterGuide{Name: "空角色"}, false); got != "" {
		t.Fatalf("没有任何槽位时应返回空串，实际：%s", got)
	}
}

// 来源三件套**不得**出现在注入文本里（提示词 v11 第 9/11/39 条禁止"社区口径 / 以官方为准"✓）
func TestRenderGuideNeverLeaksSource(t *testing.T) {
	row := fullGuideRow()
	row.SourceURL = "https://wiki.biligame.com/ys/xxx"
	row.SourceRev = "671461"
	row.Graduation = "毕业文本"
	if got := renderGuide(row, false); strings.Contains(got, "biligame") || strings.Contains(got, "wiki") {
		t.Fatalf("注入文本里不该出现来源 URL，实际：%s", got)
	}
}

// 多形态角色（旅行者）必须**在标题里标出形态** ✗ —— 不标的话模型会把冰主的毕业线当草主讲 ✓
// （2026-10-14：旅行者 8 个形态拆开存之后，标题是唯一的区分手段 ✓）
func TestRenderGuideShowsFormElement(t *testing.T) {
	row := fullGuideRow()
	row.Name = "旅行者"
	row.FormElement = "冰"
	row.Graduation = "攻击力：2800以上"
	got := renderGuide(row, false)
	if !strings.Contains(got, "【角色攻略】旅行者（冰）") {
		t.Fatalf("多形态角色的标题必须带形态，实际：%s", got)
	}
	if !strings.Contains(got, "毕业线：攻击力：2800以上") {
		t.Fatalf("毕业线应照常注入，实际：%s", got)
	}
}

// 普通角色（formElement 为空）标题保持原样 —— 不带括号 ✓（别给 120 位角色都加个空括号 ✗）
func TestRenderGuideNoFormSuffixForNormal(t *testing.T) {
	if got := renderGuide(fullGuideRow(), false); !strings.Contains(got, "【角色攻略】测试角色\n") {
		t.Fatalf("普通角色的标题不该有括号，实际：%s", got)
	}
}

// formRows 造一组形态行（火 / 冰 / 草 ✓）
func formRows() []model.CharacterGuide {
	return []model.CharacterGuide{
		{Name: "旅行者", FormElement: "草", Graduation: "草主：充能210%"},
		{Name: "旅行者", FormElement: "冰", Graduation: "冰主：攻击2800"},
		{Name: "旅行者", FormElement: "火", Graduation: "火主：攻击2000"},
	}
}

// ① 提问点名形态 ⇒ 只给那一个 ✓（用户最常说的是「火主」「冰主」✓）
func TestPickGuideRowsByAlias(t *testing.T) {
	for q, want := range map[string]string{
		"火主怎么配队？":      "火",
		"冰主的毕业面板是多少":   "冰",
		"草元素旅行者需要多少充能": "草",
	} {
		got := pickGuideRows(formRows(), q, 1)
		if len(got) != 1 || got[0].FormElement != want {
			t.Fatalf("提问 %q 应只给形态 %s，实际：%+v", q, want, got)
		}
	}
}

// ② 没点名 ⇒ 按用户定的优先级给：**火、冰**（用的人最多 ✓）
// ③ 只提了旅行者自己 ⇒ 给 2 张；同时提了别的角色 ⇒ 只给 1 张（把额度留给那个角色 ✓）
func TestPickGuideRowsDefaultPriority(t *testing.T) {
	got := pickGuideRows(formRows(), "旅行者的毕业面板是多少？", 1)
	if len(got) != 2 || got[0].FormElement != "火" || got[1].FormElement != "冰" {
		t.Fatalf("应给火、冰两张且顺序固定，实际：%+v", got)
	}

	one := pickGuideRows(formRows(), "旅行者和胡桃怎么配队？", 2)
	if len(one) != 1 || one[0].FormElement != "火" {
		t.Fatalf("同一次还提到别的角色时只给 1 张（火 ✓），实际：%+v", one)
	}
}

// ④ 普通角色（只有 1 行）行为**完全不变** ✓ —— 这条是"别把 124 位角色一起改了"的回归 ✓
func TestPickGuideRowsSingleRowUnchanged(t *testing.T) {
	rows := []model.CharacterGuide{{Name: "胡桃", FormElement: "", Graduation: "毕业"}}
	got := pickGuideRows(rows, "胡桃怎么练？", 1)
	if len(got) != 1 || got[0].Name != "胡桃" {
		t.Fatalf("单行角色必须原样返回，实际：%+v", got)
	}
}

// ⑤ 数据库返回顺序不保证 ⇒ 结果必须稳定（同一问题两次一样 ✓）；点名了形态但库里没有该形态 ⇒ 退回主形态而不是给空 ✗
func TestPickGuideRowsStableAndFallback(t *testing.T) {
	rows := formRows()
	rows = []model.CharacterGuide{rows[2], rows[0], rows[1]} // 故意打乱 ✓
	first := pickGuideRows(append([]model.CharacterGuide{}, rows...), "旅行者的毕业面板？", 1)
	second := pickGuideRows(append([]model.CharacterGuide{}, rows...), "旅行者的毕业面板？", 1)
	if len(first) != 2 || first[0].FormElement != "火" || first[1].FormElement != "冰" {
		t.Fatalf("乱序输入也必须稳定给火、冰，实际：%+v", first)
	}
	if second[0].FormElement != first[0].FormElement {
		t.Fatalf("同一提问两次结果不一致：%+v vs %+v", first, second)
	}

	missing := pickGuideRows([]model.CharacterGuide{{Name: "旅行者", FormElement: "草", Graduation: "草主"}}, "冰主怎么练？", 1)
	if len(missing) != 1 || missing[0].FormElement != "草" {
		t.Fatalf("点名的形态没有卡时应退回主形态（不能给空 ✗），实际：%+v", missing)
	}
}
