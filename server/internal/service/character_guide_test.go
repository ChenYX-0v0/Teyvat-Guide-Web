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
