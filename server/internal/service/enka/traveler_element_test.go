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
	// 目前**唯一**有实测锚点的一格：705 = 冰
	// （UID 197794747：用户自述冰主 + Enka 重拉两次均为 705）
	if got := elementFromSkillDepot(705); got != "冰" {
		t.Errorf("depot 705 应为 冰，实际 %q", got)
	}
	// ⚠️ 没有锚点的格子必须返回空 —— **不许猜** ✗
	// 教训：曾按"元素上线顺序"把 501~507/701~707 全表写死，结果与真实账号（冰主=705）不符 ✗；
	// 猜错会静默改错水主/风主玩家的元素，所以一律留空、保持既有行为。
	for _, depot := range []int{
		0, 101, 501, 502, 503, 504, 505, 506, 507,
		701, 702, 703, 704, 706, 707,
		2201, 5801, 12001, 508, 708, 5010, 7070,
	} {
		if got := elementFromSkillDepot(depot); got != "" {
			t.Errorf("depot %d 没有实测锚点，应返回空串（不许猜 ✗），实际 %q", depot, got)
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
		{10000007, 705, "冰"}, // 实测：UID 197794747 的荧（用户自述冰主，Enka skillDepotId=705）
		{10000007, 701, ""},  // 701 等其余槽位**没有实测锚点** ⇒ 不覆盖，保持卡片原值 ✓
		{10000007, 0, ""},    // depot 缺失（老数据/异常）⇒ 不改写
		{10000006, 705, ""},  // ⚠️ 10000006 是**丽莎**不是旅行者 ⇒ 永不改写 ✗
		{10000002, 705, ""},  // 非旅行者，即使 depot 巧合同号也不改写
		{10000117, 705, "冰"}, // 奇偶·男性同属旅行者系 ✓
	}
	for _, c := range cases {
		if got := travelerElementOverride(c.avatarID, c.depot); got != c.want {
			t.Errorf("avatar %d depot %d：应为 %q，实际 %q", c.avatarID, c.depot, c.want, got)
		}
	}
}

func TestTravelerAvatarIDs(t *testing.T) {
	// 旅行者系 = 空(10000005) / 荧(10000007) / 奇偶·男性(10000117) / 奇偶·女性(10000118)
	for _, id := range []int64{10000005, 10000007, 10000117, 10000118} {
		if !travelerAvatarIDs[id] {
			t.Errorf("%d 应属旅行者系", id)
		}
	}
	// ⚠️ 10000006 是**丽莎**，绝不能进这个集合 ✗（2026-09-27 我把荧记成了 10000006）
	for _, id := range []int64{10000002, 10000003, 10000006, 10000120, 0} {
		if travelerAvatarIDs[id] {
			t.Errorf("%d 不是旅行者，不应命中（10000006 = 丽莎 ✗）", id)
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
		{"荧·草主（depot 修正后为草）", dto.CharacterCard{ID: 10000007, Element: "草"}, "草"},
		{"荧·冰主（实测锚点：Enka depot=705）", dto.CharacterCard{ID: 10000007, Element: "冰"}, "冰"},
		{"空·水主", dto.CharacterCard{ID: 10000005, Element: "水"}, "水"},
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
