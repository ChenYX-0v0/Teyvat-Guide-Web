package service

// 角色机制卡注入（阶段②，设计见 teyvat-docs/ai/07 TODO-20）。
//
// 与「攻略卡」（character_guide.go）的分工：
//   · 攻略卡 = 攻略页的**结论**（配队思路 / 毕业线 / 天赋顺序）
//   · 机制卡 = 从攻略页抽出的**事实**（提供什么 / 做不到什么 / 需要什么 / 边界 / 冲突）
//
// 为什么要把事实单独抽出来 ✗：抽之前模型只能背某一页的固定配队说法（换个角度问就答歪 ✓）；
// 有了事实它就能**自己推**「能不能进这队 / 为什么不能换」✓。
//
// ⚠️ 两条硬约束（与攻略卡一致）：
//  1. **直查库、不进 asset 的内存缓存** ✗ ⇒ 后台改完立即生效 ✓
//  2. **evidence / 来源一律不进注入文本** ✗（提示词 v12 第 40~42 条：这些块是"数据" ✓）

import (
	"encoding/json"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/service/asset"
)

const (
	// maxMechanicCharacters 一次最多注入几张机制卡（与攻略卡的 maxGuideCharacters 对齐 ✓）
	maxMechanicCharacters = 2
	// maxMechanicCharsPerCard 单张卡上限（中文 1 字符 ≈ 1 token 粗估 ✓）
	//   按优先级从后往前丢（见 renderMechanic 的段顺序 ✓），不切句子中间 ✗
	maxMechanicCharsPerCard = 1100
	// maxMechanicCharsTotal 机制卡整段上限（两张卡加起来 ✓）
	maxMechanicCharsTotal = 1800
	// maxMechanicRules 一次最多注入几条全局规则 ✓
	maxMechanicRules = 3
	// maxMechanicRulesChars 全局规则整段上限 ✓（规则正文比角色卡长，单独设额度 ✓）
	//
	// ⚠️ 2026-10-14 由 800 → **1400** → **1900** ✓：新增的**反应体系**类规则（月曜 / 星烁 / 乘区 / 配队判据 ✓）
	//   单条就有 400~800 字 ✓，旧的 800 只塞得下一条多一点 ✗，而且**放不下是"跳过整条"** ✗
	//   （见 rulesForQuestion 里的 continue ✓）⇒ 会出现"资料明明在库里、却一条都没注入"的**静默丢失** ✗
	//   （实测：855 字的 `rule-reaction-systems` 直接进不去 ✓）。
	//   ⚠️ 1900 的来由 ✓：一个"体系类问题"（如「月绽放怎么组」）会**同时命中 3 条**
	//   （反应两大类 466 ＋ 月曜/星烁含开门人名单 ~880 ＋ 配队判据 ~560 ≈ 1900 ✓）
	//   ⇒ 额度不够就会把**配队判据**那条挤掉 ✗（2026-10-14 实测：「月绽放怎么组」只剩 2 条 ✓）。
	//   配套：条数 2 → 3 ✓、`cmd/mechanicsimport` 加"规则长度自检"警告 ✓。
	maxMechanicRulesChars = 2400
	// MechanicRuleCapChars 供**离线工具**自检用（导入时提示"这条太长会被跳过" ✓）——
	//   ⚠️ 别再写一份数字 ✗，一律引用这里 ✓
	MechanicRuleCapChars = maxMechanicRulesChars

	// mechanicIncludeDraft 是否注入**非人工**的机制卡（review=draft 初稿 / review=auto 自动抽取 ✓）。
	// 阶段③ 铺全量后的口径：**注入** ✓。
	//
	// 依据（2026-10-13）：自动卡由模型从攻略页抽取 ✓，但**每条都带 evidence 且经逐字校验**
	// （引用必须能在页面里原样找到 ✓，编的内容进不来 ✓）；不完整是良性的 ✓
	// （提示词 v12 第 41 条：块里没写的不许用常识补 ✗）。
	// ⚠️ 万一出现质量事故，把它改成 false ✓ ⇒ **只注入 review=reviewed 的人工卡**（一步回退 ✓）。
	mechanicIncludeDraft = true
)

// MechanicHit 一次机制注入的结果。
type MechanicHit struct {
	// Text 可直接拼进 summary 的文本（角色卡 + 命中的全局规则 ✓）
	Text string
	// Characters 本次带上了机制卡的角色名 —— 攻略卡据此**去掉「配队思路」结论** ✗：
	// 有事实就不必再喂结论（两处都写还会打架 ✓，见 ai/07 阶段③ ✓）
	Characters []string
	// Rules 命中的全局规则 key（日志用 ✓）
	Rules []string
}

// MechanicStore 机制卡查询（仅持有 *gorm.DB、不缓存 ✓）
type MechanicStore struct {
	db *gorm.DB
}

// NewMechanicStore db 为 nil 时返回 nil（调用方据此跳过 —— 与其它可选能力同样的降级方式 ✓）
func NewMechanicStore(db *gorm.DB) *MechanicStore {
	if db == nil {
		return nil
	}
	return &MechanicStore{db: db}
}

// ForQuestion 按提问里命中的角色名查机制卡，再按关键词补上全局规则，拼成一段可注入文本。
// 命中不到返回零值 ✓ —— 上层照旧走原逻辑，不会因为"没有机制卡"就答不出 ✓。
func (m *MechanicStore) ForQuestion(assets *asset.Service, question string) MechanicHit {
	if m == nil || m.db == nil || assets == nil {
		return MechanicHit{}
	}

	hit := MechanicHit{}
	blocks := make([]string, 0, maxMechanicCharacters)
	total := 0
	for _, c := range assets.CharactersMentioned(question) {
		if len(hit.Characters) >= maxMechanicCharacters || total >= maxMechanicCharsTotal {
			break
		}
		// ⚠️ 用 Find 而不是 First：First 查不到会打红色错误日志 ✗，而"这位还没机制卡"是正常情况 ✓
		query := m.db.Where("character_id = ?", c.ID)
		if !mechanicIncludeDraft {
			// 回退口径：**只认人工确认过的卡** ✓
			// （review 有三态：reviewed=人工确认 ✓ / draft=人工初稿 ✓ / auto=模型抽取 ✓）
			query = query.Where("review = ?", "reviewed")
		}
		var row model.CharacterMechanic
		if err := query.Limit(1).Find(&row).Error; err != nil || row.ID == 0 {
			continue
		}
		budget := maxMechanicCharsPerCard
		if left := maxMechanicCharsTotal - total; left < budget {
			budget = left
		}
		if budget <= 0 {
			break
		}
		block := renderMechanic(row, budget)
		if block == "" {
			continue
		}
		blocks = append(blocks, block)
		total += len([]rune(block))
		hit.Characters = append(hit.Characters, row.Name)
	}

	rules, keys := m.rulesForQuestion(question)
	if rules != "" {
		blocks = append(blocks, rules)
	}
	hit.Rules = keys
	hit.Text = strings.Join(blocks, "\n")
	return hit
}

// rulesForQuestion 按触发词挑出要注入的全局规则（未确认的一律不注入 ✗）。
func (m *MechanicStore) rulesForQuestion(question string) (string, []string) {
	q := strings.ToLower(question)
	// 规则表很小（个位数行 ✓）、每问直查 ✓ ⇒ 与角色卡同一口径，改完立即生效 ✓
	var rows []model.MechanicRule
	if err := m.db.Where("confirmed = ?", true).Order("id asc").Find(&rows).Error; err != nil {
		return "", nil
	}

	blocks := make([]string, 0, maxMechanicRules)
	keys := make([]string, 0, maxMechanicRules)
	used := 0
	for _, r := range rows {
		if len(blocks) >= maxMechanicRules || used >= maxMechanicRulesChars {
			break
		}
		if !triggersMatched(q, r.Triggers) {
			continue
		}
		line := ruleLine(r.Title, r.Content, r.RuleKey)
		cost := len([]rune(line))
		// 单条放不下就**跳过**（不截断句子中间 ✗）—— 后面的规则还有机会 ✓
		if used+cost > maxMechanicRulesChars {
			continue
		}
		blocks = append(blocks, line)
		keys = append(keys, r.RuleKey)
		used += cost
	}
	return strings.Join(blocks, "\n"), keys
}

// ruleLine 一条规则渲染成注入文本（**格式只此一份** ✗ —— 离线自检也走这里 ✓）
func ruleLine(title, content, key string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		t = key
	}
	line := "【机制规则】" + t
	if body := cleanFactText(content); body != "" {
		line += "\n· " + body
	}
	return line
}

// RuleInjectionLen 按注入格式算一条规则的字符数 ✓ ——
//
//	给 `cmd/mechanicsimport` 自检用：超过 `MechanicRuleCapChars` 的规则注入时会被**整条跳过** ✗
//	（2026-10-14 实测踩到：855 字的反应体系规则一直进不去，而且**没有任何报错** ✗）
func RuleInjectionLen(title, content, key string) int {
	return len([]rune(ruleLine(title, content, key)))
}

// triggersMatched 触发词命中判定：逗号（中英文都认 ✓）分隔，任一命中即算 ✓。
func triggersMatched(lowerQuestion, triggers string) bool {
	for _, t := range strings.FieldsFunc(triggers, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	}) {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" && strings.Contains(lowerQuestion, t) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- 渲染

// mechanicFacts 机制卡里真正要注入的**事实**字段（与 scripts/data/character_mechanics.json 对齐 ✓）。
// ⚠️ 刻意**没有** evidence / _review / evidenceIndex 字段 ✗ —— 那些只进表与后台 ✓。
type mechanicFacts struct {
	Provides []map[string]any `json:"provides"`
	// Points 与队友/队伍的联动要点（**自动卡的散文句** ✓；人工卡一般不用它 ✓）
	Points      []string         `json:"points"`
	Cannot      []string         `json:"cannot"`
	Limits      []string         `json:"limits"`
	Needs       []string         `json:"needs"`
	Scaling     []string         `json:"scaling"`
	Conflicts   []string         `json:"conflicts"`
	Shield      []map[string]any `json:"shield"`
	Interrupt   []map[string]any `json:"interruptResist"`
	Application []map[string]any `json:"application"`
	Resonance   map[string]any   `json:"resonance"`
}

// renderMechanic 把一张机制卡渲染成注入文本。
//
// 段顺序 = **优先级**（越靠前越优先；超上限时从后往前丢 ✓）：
// 「能不能进这队 / 提供什么 / 缺什么」最能改变答案 ⇒ 提供 > 做不到 > 要点 > 需要 > 边界 >
// 挂元素 > 共鸣 > 抗打断 > 护盾 > 冲突 > 基数。
func renderMechanic(row model.CharacterMechanic, budget int) string {
	var f mechanicFacts
	if err := json.Unmarshal([]byte(row.Facts), &f); err != nil {
		return "" // 数据坏了宁可不注入 ✗（半个卡片比没有更伤 ✓，同 renderGuide 的口径 ✓）
	}

	segments := []struct{ label, text string }{
		{"提供", renderProvides(f.Provides)},
		{"做不到", renderList(f.Cannot)},
		{"要点", renderList(f.Points)},
		{"需要", renderList(f.Needs)},
		{"边界", renderList(f.Limits)},
		{"挂元素", renderApplication(f.Application)},
		{"共鸣", renderResonance(f.Resonance)},
		{"抗打断", renderInterrupt(f.Interrupt)},
		{"护盾", renderShield(f.Shield)},
		{"冲突", renderList(f.Conflicts)},
		{"基数", renderList(f.Scaling)},
	}

	var b strings.Builder
	b.WriteString("【角色机制】" + row.Name)
	used := len([]rune(b.String()))
	for _, s := range segments {
		text := strings.TrimSpace(s.text)
		if text == "" {
			continue
		}
		line := "\n· " + s.label + "：" + text
		cost := len([]rune(line))
		if used+cost > budget {
			break // 这段放不下：更低的优先级也不必再试（列表是降序的 ✓）
		}
		b.WriteString(line)
		used += cost
	}
	// 一条事实都没有（数据异常）⇒ 不注入，避免只给一个光秃秃的标题 ✗
	if used <= len([]rune("【角色机制】"+row.Name)) {
		return ""
	}
	return b.String()
}

func renderProvides(items []map[string]any) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		kind := cleanFactText(firstNonEmpty(factString(it["kind"]), factString(it["kind_en"])))
		if kind == "" {
			continue
		}
		detail := make([]string, 0, 4)
		for _, k := range []struct{ key, label string }{
			{"to", "作用对象"},
			{"scale", "基数"},
			{"condition", "条件"},
			{"duration", "时长"},
			{"elements", "元素"},
			{"pattern", "形态"},
			{"note", "说明"},
		} {
			if v := cleanFactText(factString(it[k.key])); v != "" {
				detail = append(detail, k.label+"："+v)
			}
		}
		if len(detail) > 0 {
			parts = append(parts, kind+"（"+strings.Join(detail, "；")+"）")
		} else {
			parts = append(parts, kind)
		}
	}
	return strings.Join(parts, "｜")
}

func renderApplication(items []map[string]any) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		element := cleanFactText(factString(it["element"]))
		pattern := cleanFactText(factString(it["pattern"]))
		switch {
		case element != "" && pattern != "":
			parts = append(parts, element+"｜"+pattern)
		case element != "":
			parts = append(parts, element)
		case pattern != "":
			parts = append(parts, pattern)
		}
	}
	return strings.Join(parts, "；")
}

func renderResonance(node map[string]any) string {
	if len(node) == 0 {
		return ""
	}
	parts := make([]string, 0, 2)
	if req := renderList(factStrings(node["requires"])); req != "" {
		parts = append(parts, "需凑出："+req)
	}
	if pro := renderList(factStrings(node["provides"])); pro != "" {
		parts = append(parts, "能促成："+pro)
	}
	return strings.Join(parts, "；")
}

func renderInterrupt(items []map[string]any) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		who := cleanFactText(factString(it["to"]))
		coef := factString(it["coefficient"])
		cond := cleanFactText(factString(it["condition"]))
		head := who
		if head == "" {
			head = "抗打断"
		}
		if coef != "" {
			head += " 系数 " + coef
		}
		if cond != "" {
			head += "（" + cond + "）"
		}
		parts = append(parts, head)
	}
	return strings.Join(parts, "；")
}

func renderShield(items []map[string]any) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		who := cleanFactText(factString(it["to"]))
		cond := cleanFactText(factString(it["condition"]))
		value := cleanFactText(factString(it["value"]))
		head := who
		if head == "" {
			head = "护盾"
		}
		if value != "" {
			head += " " + value
		}
		if cond != "" {
			head += "（" + cond + "）"
		}
		parts = append(parts, head)
	}
	return strings.Join(parts, "；")
}

func renderList(items []string) string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if v := cleanFactText(s); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, "；")
}

// factStrings 把 JSON 里的任意值转成字符串列表（`elements: ["火","水"]` / 单个字符串都认 ✓）
func factStrings(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return []string{s}
		}
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := factString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	case bool:
		if t {
			return []string{"是"}
		}
		return []string{"否"}
	default:
		return []string{factString(v)}
	}
}

// factString 单值 → 字符串（数字也认 ✓：抗打断系数 0.5 ✓）
func factString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		// JSON 数字一律是 float64 ⇒ 整数别渲染成 1.000000、0.5 别渲染成 0.500000 ✗
		// （抗打断系数 0.5 / 0 要原样出现 ✓）
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "是"
		}
		return "否"
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := factString(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "、")
	default:
		return ""
	}
}

// cleanFactText 去掉 Markdown 强调符与多余空白 —— 注入文本要干净 ✓
// （提示词 v11 第 14 条禁止回复里出现 Markdown；数据块里也别留 `**` ✓）
func cleanFactText(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.TrimSpace(s)
	return s
}
