package enka

import (
	"testing"

	"teyvat-guide/server/internal/dto"
)

// 旅行者七元素接 Enka skillDepotId（2026-09-27）。
//
// 背景：角色表里旅行者只能写死一个元素 ✗ —— 玩家换成草主/水主后，
// 元素伤害加成、面板着色、(角色,元素) 的小助手权重会全错。修法是读 Enka 的 skillDepotId。

func TestElementFromSkillDepot(t *testing.T) {
	// 男（空）501~507 与 女（荧）701~707，同序：风 / 岩 / 雷 / 草 / 水 / 火 / 冰
	want := map[int]string{
		501: "风", 502: "岩", 503: "雷", 504: "草", 505: "水", 506: "火", 507: "冰",
		701: "风", 702: "岩", 703: "雷", 704: "草", 705: "水", 706: "火", 707: "冰",
	}
	for depot, elem := range want {
		if got := elementFromSkillDepot(depot); got != elem {
			t.Errorf("depot %d 应为 %s，实际 %q", depot, elem, got)
		}
	}
	// 非旅行者（普通角色的技能套件 id）必须返回空 —— 否则会给别人乱改元素 ✗
	for _, depot := range []int{0, 101, 5010, 7070, 508, 708, 1001} {
		if got := elementFromSkillDepot(depot); got != "" {
			t.Errorf("depot %d 非旅行者，应返回空串，实际 %q", depot, got)
		}
	}
}

// 接线测试：translate 循环靠这个函数决定要不要改写元素
// （防"映射写对了，但没接上去" ✗）
func TestTravelerElementOverride(t *testing.T) {
	cases := []struct {
		avatarID int64
		depot    int
		want     string
	}{
		{10000007, 705, "水"}, // 实测样本：UID 197794747 的旅行者（水主，Enka skillDepotId=705）
		{10000006, 704, "草"}, // 荧 · 草主
		{10000005, 501, "风"}, // 空 · 风主
		{10000117, 707, "冰"},
		{10000007, 0, ""},   // depot 缺失（老数据/异常）⇒ 不改写，保持卡片原值
		{10000002, 501, ""}, // 非旅行者，即使 depot 巧合落在区间内也不改写
	}
	for _, c := range cases {
		if got := travelerElementOverride(c.avatarID, c.depot); got != c.want {
			t.Errorf("avatar %d depot %d：应为 %q，实际 %q", c.avatarID, c.depot, c.want, got)
		}
	}
}

func TestTravelerAvatarIDs(t *testing.T) {
	for _, id := range []int64{10000005, 10000006, 10000007, 10000117, 10000118} {
		if !travelerAvatarIDs[id] {
			t.Errorf("%d 应属旅行者系（含荧 10000006 —— 之前漏了 ✗）", id)
		}
	}
	for _, id := range []int64{10000002, 10000003, 0, 10000120} {
		if travelerAvatarIDs[id] {
			t.Errorf("%d 不是旅行者，不应命中", id)
		}
	}
}

// travelerElement：只认旅行者 + 七元素白名单；其余一律 ""（⇒ 退回通用权重行，不会算错）
func TestTravelerElement(t *testing.T) {
	cases := []struct {
		name string
		card dto.CharacterCard
		want string
	}{
		{"草主（depot 已修正为草）", dto.CharacterCard{ID: 10000006, Element: "草"}, "草"},
		{"水主", dto.CharacterCard{ID: 10000005, Element: "水"}, "水"},
		{"非旅行者即便元素合法也不认", dto.CharacterCard{ID: 10000002, Element: "冰"}, ""},
		{"旅行者但元素为空（拿不到可信值）", dto.CharacterCard{ID: 10000005, Element: ""}, ""},
		{"旅行者但元素不是七元素", dto.CharacterCard{ID: 10000117, Element: "未知"}, ""},
	}
	for _, c := range cases {
		if got := travelerElement(&c.card); got != c.want {
			t.Errorf("%s：应为 %q，实际 %q", c.name, c.want, got)
		}
	}
}
