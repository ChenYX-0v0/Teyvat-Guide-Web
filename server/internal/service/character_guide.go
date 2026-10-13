package service

// 角色攻略卡（阶段②，见 teyvat-docs/ai/07 TODO-18）：
// 让「配队思路 / 毕业线 / 天赋顺序」**有据可依** —— 这三类此前库里一条都没有 ✗，
// 模型只能"没有查到"✗ 或凭记忆泛谈 ✗。
//
// ⚠️ 与其他资产不同，攻略卡**不走 asset.Service 的内存缓存** ✗：每问直查库
//   （character_id 唯一索引 ≈ 亚毫秒 ✓）⇒ 后台改完立即生效、不用 reload ✓，
//   也绕开「两个 asset.Service 实例」那个老坑 ✓。

import (
	"strings"

	"gorm.io/gorm"

	"teyvat-guide/server/internal/model"
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
)

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
func (g *GuideStore) ForQuestion(assets *asset.Service, question string) string {
	if g == nil || g.db == nil || assets == nil {
		return ""
	}
	chars := assets.CharactersMentioned(question)
	if len(chars) == 0 {
		return ""
	}

	blocks := make([]string, 0, maxGuideCharacters)
	total := 0
	for _, c := range chars {
		if len(blocks) >= maxGuideCharacters || total >= maxGuideCharsTotal {
			break
		}
		var row model.CharacterGuide
		if err := g.db.Where("character_id = ?", c.ID).First(&row).Error; err != nil {
			continue // 这位没有攻略卡 ⇒ 跳过（不是错误 ✓）
		}
		block := renderGuide(row)
		if block == "" {
			continue
		}
		blocks = append(blocks, block)
		total += len([]rune(block))
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
func renderGuide(row model.CharacterGuide) string {
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

	var b strings.Builder
	b.WriteString("【角色攻略】" + row.Name)
	used := len([]rune(b.String()))
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
	if used <= len([]rune("【角色攻略】"+row.Name)) {
		return ""
	}
	return b.String()
}
