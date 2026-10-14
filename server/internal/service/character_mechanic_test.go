package service

// 机制卡的回归测试（阶段②，TODO-20）。
// 这里钉的是四件容易悄悄坏掉的事 ✓：
//   ① 注入文本里**绝不能出现 evidence / 来源** ✗（提示词 v12 第 40~42 条要求这些块只是"数据"）
//   ② 段顺序 = 优先级（超预算时丢的是最不重要的那段 ✓）
//   ③ 全局规则**只在触发词命中且 confirmed=true** 时才注入 ✓
//   ④ 有机制卡的角色，攻略卡要**去掉「配队思路」结论** ✓（事实优先，避免两处打架 ✓）

import (
	"strconv"
	"strings"
	"testing"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
)

func sampleFacts() string {
	return `{
	  "provides": [
	    {"kind": "攻击力加成", "to": "领域内站场角色", "scale": "自身基础攻击力",
	     "condition": "元素爆发「美妙旅程」形成的鼓舞领域内",
	     "evidence": "角色定位：形成的鼓舞领域可根据班尼特的基础攻击力为领域内站场角色提供高额的攻击力加成"},
	    {"kind": "治疗", "to": "场上角色", "condition": "仅生命值低于 70% 时、单体",
	     "evidence": "角色定位：还能为前台提供每秒一跳的治疗"}
	  ],
	  "cannot": ["不能降低风元素抗性"],
	  "needs": ["元素充能效率"],
	  "limits": ["治疗是单体且仅在生命值低于 70% 时触发"],
	  "resonance": {"requires": ["另一名火元素角色"], "provides": ["可凑双火共鸣"]},
	  "interruptResist": [{"to": "自身", "coefficient": 0.5, "condition": "第一命座解锁后"}],
	  "scaling": ["攻击力"],
	  "evidenceIndex": {"page": "班尼特/攻略"}
	}`
}

// ① 段顺序 + evidence 不外泄 + 数值格式
func TestRenderMechanicOrderAndNoEvidence(t *testing.T) {
	got := renderMechanic(model.CharacterMechanic{Name: "测试角色", Facts: sampleFacts()}, maxMechanicCharsPerCard)

	want := []string{
		"【角色机制】测试角色",
		"· 提供：",
		"攻击力加成（作用对象：领域内站场角色；基数：自身基础攻击力；条件：元素爆发「美妙旅程」形成的鼓舞领域内）",
		"· 做不到：",
		"· 需要：",
		"· 边界：",
		"· 共鸣：",
		"· 抗打断：",
		"· 基数：",
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

	// evidence 与页码索引**一律不得出现** ✗
	for _, leak := range []string{"evidence", "角色定位：", "班尼特/攻略", "可根据班尼特的基础攻击力"} {
		if strings.Contains(got, leak) {
			t.Fatalf("注入文本里不该出现 %q\n实际：%s", leak, got)
		}
	}
	// 抗打断系数必须原样（0.5 ✓，不能是 0.500000 / 0 ✗）
	if !strings.Contains(got, "系数 0.5") {
		t.Fatalf("抗打断系数应渲染成 0.5，实际：%s", got)
	}
}

// ② 超上限时**从低优先级丢**：提供 / 做不到 / 需要 / **冲突** 必须留下，「基数」先被丢 ✓
//
// ⚠️ 2026-10-14 改（配合段顺序调整 ＋ 单条截断 ✓）：
//
//	旧版靠"塞一条 2000 字的基数"逼出裁剪 ✗ —— 现在那条只会被**截断到 80 字** ✓、不再消失 ✓
//	（这正是本轮想要的行为：截断一条，好过整段被丢 ✗）。改成把前面几段都塞满 ✓。
func TestRenderMechanicDropsLowPriorityFirst(t *testing.T) {
	filler := strings.Repeat("填", 75) // 每条 76 字 ⇒ 渲染后 80 字封顶 ✓
	item := func(n int, tag string) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = `"` + filler + tag + strconv.Itoa(i) + `"`
		}
		return strings.Join(parts, ",")
	}
	facts := `{
	  "provides":  [{"kind": "` + filler + `"}],
	  "cannot":    [` + item(4, "a") + `],
	  "needs":     [` + item(4, "b") + `],
	  "conflicts": [` + item(4, "c") + `],
	  "points":    [` + item(4, "d") + `],
	  "limits":    [` + item(4, "e") + `],
	  "scaling":   ["` + filler + `"]
	}`

	got := renderMechanic(model.CharacterMechanic{Name: "测试角色", Facts: facts}, maxMechanicCharsPerCard)
	if strings.Contains(got, "· 基数：") {
		t.Fatalf("超限时应先丢掉最低优先级的「基数」，实际仍包含：%s", got[:min(80, len(got))])
	}
	// 前四段（回答"这队怎么搭 / 谁能进队"的那四段 ✓）必须留下 ✓
	for _, keep := range []string{"· 提供：", "· 做不到：", "· 需要：", "· 冲突"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("超限时不该丢 %q\n实际：%s", keep, got)
		}
	}
}

// ③ 单段最多 4 条 ✓ ＋ 超长条目截断加「…」✓ —— 防"一段吃光整张卡" ✗（2026-10-14 加 ✓）
func TestRenderMechanicItemCapAndTruncate(t *testing.T) {
	long := strings.Repeat("长", 200)
	facts := `{"cannot": ["` + long + `","短二","短三","短四","短五","短六"]}`
	got := renderMechanic(model.CharacterMechanic{Name: "封顶", Facts: facts}, maxMechanicCharsPerCard)

	if n := strings.Count(got, "；"); n != maxMechanicItemsPerSegment-1 {
		t.Fatalf("单段应只渲染 %d 条（分隔符 %d 个），实际分隔符 %d 个：%s",
			maxMechanicItemsPerSegment, maxMechanicItemsPerSegment-1, n, got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("超长条目应被截断并加「…」✓，实际：%s", got)
	}
	for _, dropped := range []string{"短五", "短六"} {
		if strings.Contains(got, dropped) {
			t.Fatalf("第 %d 条起不该渲染 ✗，实际：%s", maxMechanicItemsPerSegment+1, got)
		}
	}
	// 截断长度：200 字的条目应只剩 80 字 ＋「…」✓
	if !strings.Contains(got, strings.Repeat("长", maxMechanicItemChars)+"…") {
		t.Fatalf("截断长度应为 %d 字 ＋「…」✗，实际：%s", maxMechanicItemChars, got)
	}
}

// 一条事实都没有（或 JSON 坏了）⇒ 返回空串：**宁可不注入**，也不要一个光秃秃的标题 ✗
func TestRenderMechanicEmptyOrBroken(t *testing.T) {
	if got := renderMechanic(model.CharacterMechanic{Name: "空角色", Facts: "{}"}, 500); got != "" {
		t.Fatalf("没有事实时应返回空串，实际：%s", got)
	}
	if got := renderMechanic(model.CharacterMechanic{Name: "坏角色", Facts: "{不是 JSON"}, 500); got != "" {
		t.Fatalf("JSON 坏了应返回空串（半个卡片比没有更伤 ✓），实际：%s", got)
	}
}

// ③ 触发词匹配：中英文逗号都认 ✓、大小写不敏感 ✓、空触发词永不注入 ✓
func TestTriggersMatched(t *testing.T) {
	cases := []struct {
		q, triggers string
		want        bool
	}{
		{"减抗能叠加吗", "减抗,抗性", true},
		{"风套和钟离能一起带吗", "减抗，风套", true},
		{"Reaction减抗", "减抗", true},
		{"班尼特怎么练", "减抗,抗性", false},
		{"减抗能叠加吗", "", false},
		{"减抗能叠加吗", " , ，", false},
	}
	for _, c := range cases {
		if got := triggersMatched(strings.ToLower(c.q), c.triggers); got != c.want {
			t.Fatalf("triggersMatched(%q, %q) = %v，期望 %v", c.q, c.triggers, got, c.want)
		}
	}
}

// ③ 规则注入：**只有 confirmed=true 且触发词命中**才带上 ✓
func TestRulesForQuestionConfirmedAndTriggered(t *testing.T) {
	setupUsageTestDB(t)
	db := database.DB()
	if err := db.AutoMigrate(&model.CharacterMechanic{}, &model.MechanicRule{}); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	rows := []model.MechanicRule{
		{RuleKey: "rule-ok", Title: "已确认规则", Content: "不同名减抗可以叠加", Triggers: "减抗,抗性", Confirmed: true},
		{RuleKey: "rule-unconfirmed", Title: "未确认规则", Content: "这条还没核实", Triggers: "减抗", Confirmed: false},
		{RuleKey: "rule-other", Title: "共鸣表", Content: "双火 +25% 攻击力", Triggers: "共鸣", Confirmed: true},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("插规则失败：%v", err)
		}
	}
	store := NewMechanicStore(db)

	text, keys := store.rulesForQuestion("减抗能叠加吗")
	if len(keys) != 1 || keys[0] != "rule-ok" {
		t.Fatalf("只应注入已确认且命中的 rule-ok，实际：%v", keys)
	}
	if !strings.Contains(text, "不同名减抗可以叠加") {
		t.Fatalf("规则正文没进去：%s", text)
	}
	if strings.Contains(text, "这条还没核实") {
		t.Fatalf("未确认的规则不得注入 ✗，实际：%s", text)
	}

	if text, keys := store.rulesForQuestion("班尼特的天赋怎么点"); text != "" || len(keys) != 0 {
		t.Fatalf("触发词不命中时不该注入，实际：%v / %s", keys, text)
	}
}

// ④ **有机制卡也要注入「配队思路」** ✓ —— 口径于 2026-10-15 反转 ✗。
//
//	旧行为（已废）：有机制卡 ⇒ 丢掉配队思路 ✗，理由是"事实优先、别抄结论" ✓；
//	实测后果 ✗：机制卡只写**职能**（"另一名雷元素队友"✗），而**带名字的名单**只在攻略卡里 ✓
//	⇒ 菲林斯配队回答里**一个队友都没有** ✗✗（用户原话：「不要让用户继续猜可以带谁」✓）。
//	⇒ 现在**两份都注入** ✓：机制卡给事实、攻略卡给名单 ✓，冲突由提示词第 46 条裁决 ✓。
func TestRenderGuideKeepsTeamEvenWithMechanicCard(t *testing.T) {
	row := fullGuideRow()
	row.Name = "菲林斯"
	row.TeamPick = "雷元素队友选择包括：*伊涅芙提供护盾与月感电反应基础伤害加成；*菲谢尔提供后台雷元素伤害。"

	got := renderGuide(row)

	if !strings.Contains(got, "· 配队思路：") {
		t.Fatalf("有机制卡也**必须**注入配队思路（「能带谁」只在这里 ✗）\n实际：%s", got)
	}
	for _, n := range []string{"伊涅芙", "菲谢尔"} {
		if !strings.Contains(got, n) {
			t.Fatalf("队友名字 %s 必须出现在注入文本里（别让用户自己猜 ✗）\n实际：%s", n, got)
		}
	}
	for _, keep := range []string{"【角色攻略】菲林斯", "· 毕业线：", "· 天赋顺序：", "· 注意事项："} {
		if !strings.Contains(got, keep) {
			t.Fatalf("其余段应照旧保留：缺 %q\n实际：%s", keep, got)
		}
	}
}
