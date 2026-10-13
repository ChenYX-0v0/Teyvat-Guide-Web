package service

// 角色攻略卡（阶段②，见 teyvat-docs/ai/07 TODO-18）：
// 让「配队思路 / 毕业线 / 天赋顺序」**有据可依** —— 这三类此前库里一条都没有 ✗，
// 模型只能"没有查到"✗ 或凭记忆泛谈 ✗。
//
// ⚠️ 与其他资产不同，攻略卡**不走 asset.Service 的内存缓存** ✗：每问直查库
//   （character_id 唯一索引 ≈ 亚毫秒 ✓）⇒ 后台改完立即生效、不用 reload ✓，
//   也绕开「两个 asset.Service 实例」那个老坑 ✓。

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gorm.io/gorm"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/service/asset"
)

const (
	// maxGuideCharacters 一次最多注入几张攻略卡（与 V2 资料的 maxReferenceCharacters 对齐 ✓）
	maxGuideCharacters = 2
	// maxGuideCharsPerCard 单张卡的字符上限（中文 1 字符 ≈ 1 token 粗估 ✓）
	//   按优先级从后往前丢（见 renderGuide 的段顺序 ✓），不切句子中间 ✗
	maxGuideCharsPerCard = 1600
	// maxGuideCharsTotal 整段攻略文本的上限（两张卡加起来 ✓）
	maxGuideCharsTotal = 2400
	// maxGuideFormsPerCharacter 多形态角色（旅行者）一次最多取几行 ✓ —— 8 篇形态页（7 元素 ＋ 原人 ✓）
	maxGuideFormsPerCharacter = 8
)

// debugGuide 形态选择的排障开关（显式设了 GUIDE_DEBUG 才打印 ✓；线上默认关 ✓）
func debugGuide() bool { return os.Getenv("GUIDE_DEBUG") != "" }

// guideFormElements 旅行者的形态元素（**顺序无关**，只用于识别提问里点名了哪个形态 ✓）
//
// 形态别名（2026-10-14 用户：「旅行者各元素都单独处理，**火主 / 冰主优先**（用的人多）」✓）：
// 玩家几乎不说"旅行者"✗，只说「火主 / 冰主」✓，或者「火元素旅行者」✓。
// ⚠️ 与 `asset.travelerFormAliases` 必须保持同一套别名 ✗（那里负责"命不命中旅行者"✓，
// 这里负责"命中之后给哪一形态"✓）—— 改一处记得改另一处 ✓。
var guideFormElements = []string{"火", "冰", "水", "草", "雷", "风", "岩"}

// defaultFormPriority 提问里看不出形态时的注入顺序 ✓
//
//	= 用户定的优先级：**火、冰**（用的人最多 ✓）；先只列这两个，超出就不猜了 ✓
var defaultFormPriority = []string{"火", "冰"}

// formFromQuestion 从提问里认出点名的形态（火主 / 冰主 / 火元素旅行者…）✓；认不出返回空串 ✓
func formFromQuestion(question string) string {
	for _, e := range guideFormElements {
		if strings.Contains(question, e+"主") || strings.Contains(question, e+"元素旅行者") {
			return e
		}
	}
	return ""
}

// formPriorityIndex 默认优先级里的次序（越小越先 ✓；没列到的排最后 ✓）
func formPriorityIndex(form string) int {
	for i, e := range defaultFormPriority {
		if e == form {
			return i
		}
	}
	return len(defaultFormPriority)
}

// pickGuideRows 从**同一角色的形态组**里挑出本次要注入的行 ✓。
// 普通角色只有 1 行 ⇒ 原样返回（行为与改动前**完全一致** ✓，不影响任何非旅行者角色 ✓）。
//
// 三步（2026-10-14 定 ✓）：
//  1. 稳定排序：先按默认优先级（火 → 冰 ✓）、再按形态名 ✗（数据库返回顺序不保证 ✓，
//     不能让它决定注入什么 ✓ —— 那样同一个问题可能两次答得不一样 ✗）；
//  2. 提问里**点名了形态** ⇒ 只给那一个 ✓（点名却查不到卡 ⇒ 退回主形态 ✓，绝不给空 ✗）；
//  3. 没点名 ⇒ 按优先级给（火、冰 ✓，最多 2 张 ✓）；但同一次提问里**还提到了别的角色** ⇒
//     只给 1 张（火 ✓），把额度留给那个角色 ✓（maxGuideCharacters=2 ✓）。
func pickGuideRows(rows []model.CharacterGuide, question string, mentionedChars int) []model.CharacterGuide {
	if len(rows) <= 1 {
		return rows
	}
	sort.SliceStable(rows, func(i, j int) bool {
		pi, pj := formPriorityIndex(rows[i].FormElement), formPriorityIndex(rows[j].FormElement)
		if pi != pj {
			return pi < pj
		}
		return rows[i].FormElement < rows[j].FormElement
	})

	if f := formFromQuestion(question); f != "" {
		out := make([]model.CharacterGuide, 0, 1)
		for _, r := range rows {
			if r.FormElement == f {
				out = append(out, r)
			}
		}
		if len(out) > 0 {
			return out
		}
		return rows[:1]
	}

	limit := 2
	if mentionedChars > 1 {
		limit = 1
	}
	out := make([]model.CharacterGuide, 0, limit)
	for _, f := range defaultFormPriority {
		for _, r := range rows {
			if r.FormElement == f {
				out = append(out, r)
			}
		}
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		return rows[:1]
	}
	return out
}

// GuideStore 攻略卡查询（仅持有 *gorm.DB，不缓存 ✓）
type GuideStore struct {
	db *gorm.DB
}

// NewGuideStore db 为 nil 时返回 nil（调用方据此跳过 —— 与其它可选能力同样的降级方式 ✓）
func NewGuideStore(db *gorm.DB) *GuideStore {
	if db == nil {
		return nil
	}
	return &GuideStore{db: db}
}

// ForQuestion 按提问里命中的角色名查攻略卡，拼成一段可直接注入的文本。
// 命中不到（或库里没这张卡）返回空串 ✓ —— 上层照旧走「没有查到」的老逻辑，不会因此答不出 ✓。
//
// dropTeam 本次**已带机制卡**的角色名（见 character_mechanic.go ✓）：这些角色的
// 「配队思路」结论**不注入** ✗ —— 机制卡给的是事实，模型自己推得比抄结论稳 ✓；
// 两处都写必然打架（同一个问题两个答案 ✓），所以以事实为准 ✓（ai/07 阶段③ 的全量口径 ✓）。
func (g *GuideStore) ForQuestion(assets *asset.Service, question string, dropTeam map[string]bool) string {
	if g == nil || g.db == nil || assets == nil {
		return ""
	}
	chars := assets.CharactersMentioned(question)
	if debugGuide() {
		fmt.Fprintf(os.Stderr, "[guide-debug] q=%q chars=%d\n", question, len(chars))
	}
	if len(chars) == 0 {
		return ""
	}

	blocks := make([]string, 0, maxGuideCharacters)
	total := 0
	for _, c := range chars {
		if len(blocks) >= maxGuideCharacters || total >= maxGuideCharsTotal {
			break
		}
		// ⚠️ 用 Find 而不是 First：First 查不到会打一条红色 `record not found` ✗，
		//   而「这位还没有攻略卡」是完全正常的情况 ✓（与 cmd/guideimport 同一口径 ✓；
		//   2026-10-13 导出机制评测上下文时实测：魈、胡桃都刷了这条红字 ✗）。
		// ⚠️ 这里**不按 character_id 取单行** ✗：多形态角色（旅行者）一个 id 有 8 行 ✓
		//   ⇒ 取回来再按形态挑（pickGuideRows ✓，火 / 冰优先 ✓）。
		// ⚠️ 按**名字**查，不按 character_id ✗（2026-10-14 修的抖动 ✓）：
		//   `characters` 表里「旅行者」有 10000005 / 10000007 **两行同名** ✓ ⇒
		//   按 id 查会随机落到没有攻略卡的那一行上 ✗ ⇒ 同一句话有时答得出、有时答不出 ✓。
		//   攻略表本来就冗余存了 `name`（见模型注释 ✓），按名字查 = 一个角色的所有形态一次取全 ✓。
		// ⚠️ 顺带**把 error 打出来** ✗（只判 RowsAffected 会把"查询失败"伪装成"没这张卡"✓）
		var rows []model.CharacterGuide
		err := g.db.Where("name = ?", c.Name).Limit(maxGuideFormsPerCharacter).Find(&rows).Error
		if err != nil {
			logger.L().Warnw("guide rows query failed", "name", c.Name, "err", err.Error())
		}
		if len(rows) == 0 {
			continue // 这位没有攻略卡（或查询失败 ✓ 已 warn）⇒ 跳过（不是错误 ✓）
		}
		picked := pickGuideRows(rows, question, len(chars))
		if debugGuide() {
			forms := make([]string, 0, len(rows))
			for _, r := range rows {
				forms = append(forms, r.FormElement)
			}
			pf := make([]string, 0, len(picked))
			for _, r := range picked {
				pf = append(pf, r.FormElement)
			}
			fmt.Fprintf(os.Stderr, "[guide-debug] %s id=%d rows=%d %v ⇒ picked=%v\n", c.Name, c.ID, len(rows), forms, pf)
		}
		for _, row := range picked {
			if len(blocks) >= maxGuideCharacters || total >= maxGuideCharsTotal {
				break
			}
			block := renderGuide(row, dropTeam[row.Name])
			if block == "" {
				continue
			}
			blocks = append(blocks, block)
			total += len([]rune(block))
		}
	}
	return strings.Join(blocks, "\n")
}

// renderGuide 把一张卡渲染成注入文本。
//
// 段顺序 = **优先级**（越靠前越优先；超上限时从后往前丢 ✓）：
// 按用户抱怨的三类排在最前 —— 配队思路 > 毕业线 > 天赋顺序 > 武器 > 圣遗物 > 定位 > 注意事项。
//
// ⚠️ 刻意**不带来源 / 口径标注** ✗：那些只进表与后台界面（提示词 v11 第 9/11/39 条禁止
// "社区口径 / 以官方为准"这类出戏说法 ✓）；配队来自反向索引时会多一个来源说明 ✓，
// 那是**事实性**的（"据其他角色攻略页"），不是免责声明 ✓。
//
// dropTeam：该角色本次已带机制卡 ⇒ 去掉「配队思路」结论（事实优先 ✓，见 ForQuestion 注释 ✓）。
func renderGuide(row model.CharacterGuide, dropTeam bool) string {
	type segment struct{ label, text string }
	segments := []segment{
		{"配队思路", row.TeamPick},
		{"毕业线", row.Graduation},
		{"天赋顺序", row.Talents},
		{"武器", row.WeaponPick},
		{"圣遗物", row.RelicPick},
		{"定位", row.Role},
		{"注意事项", row.Notes},
	}
	if dropTeam {
		segments = segments[1:] // 有机制卡 ⇒ 不再注入「配队思路」结论 ✗（其余顺序不变 ✓）
	}

	var b strings.Builder
	// 多形态角色必须**标出形态** ✗ —— 不标的话，模型会把「冰主」的毕业线当草主讲 ✓
	//   （2026-10-14：旅行者 8 个形态拆开存之后，标题是唯一的区分手段 ✓）
	title := "【角色攻略】" + row.Name
	if f := strings.TrimSpace(row.FormElement); f != "" {
		title += "（" + f + "）"
	}
	b.WriteString(title)
	used := len([]rune(title))
	for _, s := range segments {
		text := strings.TrimSpace(s.text)
		if text == "" {
			continue
		}
		label := s.label
		if label == "配队思路" && row.TeamFrom == "reverse" {
			label = "配队思路（据其他角色攻略页）"
		}
		line := "\n· " + label + "：" + text
		cost := len([]rune(line))
		if used+cost > maxGuideCharsPerCard {
			break // 这段放不下：更低的优先级也不必再试（列表是降序的 ✓）
		}
		b.WriteString(line)
		used += cost
	}
	// 连一段都放不下（数据异常）⇒ 不注入，避免只给一个光秃秃的标题 ✗
	if used <= len([]rune(title)) {
		return ""
	}
	return b.String()
}
