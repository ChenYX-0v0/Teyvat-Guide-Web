package asset

// 圣遗物评分（V3，2026-09-27）
//
// 算法复刻自 miao-plugin —— **MIT License, Copyright (c) 2023 Yoimiya**
//   · models/artis/ArtisMark.js      计分与归一化（单件满分 66、分档阈值）
//   · models/artis/ArtisMarkCfg.js   fixWeight 折算（白值按角色基础值）+ 动态规则
//   · resources/meta-gs/artifact/extra.js   档位数值（basicNum = 3.885，value = basicNum × attrPct）
//
// 设计、口径与验证结论见 teyvat-docs/ai/08-圣遗物评分设计与落地.md。
//
// ⚠️ 本文件刻意**不依赖 internal/dto**：dto 依赖 asset（CharacterCard.StatWeights），
//    反向 import 会形成循环；因此入参用下面的 RelicScoreInput 自带基础字段。
//
// 口径：
//   · Score   —— 口径 A：复刻上游（0~66，可与 miao 等工具对照）
//   · Percent —— 口径 B：本件占"该角色该部位理论上限"的百分比（0~100，更直观）
//   · Class   —— 分档（D/C/B/A/S/SS/SSS/ACE/MAX），阈值与上游一致
//
// ⚠️ 对外（前端 / AI 回答）**不做口径声明**（2026-09-27 决策）：给分数就给分数，
// 不附"社区参考/非官方"这类话术；界面只描述刻度。

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ───────────────────────── 上游常数（miao-plugin，MIT） ─────────────────────────

// relicBasicNum extra.js 的 basicNum：档位数值 = basicNum × 词条系数
const relicBasicNum = 3.885

// relicScoreScale 单件满分（ArtisMark.js: ret × 66 / posMax）
const relicScoreScale = 66.0

// 角色基础值缺省（ArtisMarkCfg.js: baseAttr || { hp: 14000, atk: 230, def: 700 }）
const (
	relicDefaultBaseHP  = 14000.0
	relicDefaultBaseATK = 230.0
	relicDefaultBaseDEF = 700.0
	// 攻击力折算附加常量（ArtisMarkCfg.js: k === 'atk' ? 520 : 0）
	relicAtkBasePlus = 520.0
)

// relicAllDims 全部评分维度（含白值维度 *Plus）
var relicAllDims = []string{
	"hp", "atk", "def", "hpPlus", "atkPlus", "defPlus",
	"cpct", "cdmg", "mastery", "recharge", "dmg", "phy", "heal",
}

// relicMaxTier 单次强化"最大档"（= basicNum × attrPct）。评分按它归一（上游口径）
var relicMaxTier = map[string]float64{
	"hp":       relicBasicNum * 1.5,        // 5.8275
	"atk":      relicBasicNum * 1.5,        // 5.8275
	"def":      relicBasicNum * 1.875,      // 7.284375
	"hpPlus":   relicBasicNum * 1.875 * 41, // 298.659375
	"atkPlus":  relicBasicNum * 5,          // 19.425
	"defPlus":  relicBasicNum * 6,          // 23.31
	"cpct":     relicBasicNum * 1,          // 3.885
	"cdmg":     relicBasicNum * 2,          // 7.77
	"mastery":  relicBasicNum * 6,          // 23.31
	"recharge": relicBasicNum / 0.6,        // 6.475
	"dmg":      relicBasicNum * 1.5,        // 5.8275（主词条，非副词条）
	"phy":      relicBasicNum * 1.875,      // 7.284375（主词条）
	"heal":     relicBasicNum / 1.3,        // 4.4826923…（主词条）
}

// relicTierScale 均值/最大档之比：上游 valueMin = value × 0.7（四档 0.7/0.8/0.9/1.0 均值 = 0.85）
const relicTierScale = 0.85

// relicAvgTier 单次强化"均值档"（= 最大档 × 0.85）。**只用于词条数展示口径**（3.3 暴击 / 6.6 暴伤 …）
var relicAvgTier = func() map[string]float64 {
	out := make(map[string]float64, len(relicMaxTier))
	for k, v := range relicMaxTier {
		out[k] = v * relicTierScale
	}
	return out
}()

// relicBaseOf 白值维度 → 所归属的百分比维度（有 base 的维度要做基础值折算）
var relicBaseOf = map[string]string{"hpPlus": "hp", "atkPlus": "atk", "defPlus": "def"}

// relicDimByName 中文词条名 → 维度 key（名字取自 cmd/assetimport/props_zh.json，勿臆造）
var relicDimByName = map[string]string{
	"生命值":     "hpPlus",
	"百分比生命值":  "hp",
	"攻击力":     "atkPlus",
	"百分比攻击力":  "atk",
	"防御力":     "defPlus",
	"百分比防御力":  "def",
	"暴击率":     "cpct",
	"暴击伤害":    "cdmg",
	"元素精通":    "mastery",
	"元素充能效率":  "recharge",
	"治疗加成":    "heal",
	"物理伤害加成":  "phy",
	"火元素伤害加成": "dmg",
	"水元素伤害加成": "dmg",
	"雷元素伤害加成": "dmg",
	"冰元素伤害加成": "dmg",
	"风元素伤害加成": "dmg",
	"岩元素伤害加成": "dmg",
	"草元素伤害加成": "dmg",
}

// relicSlotIndex 部位 → 序号（1 花 / 2 羽 / 3 沙 / 4 杯 / 5 冠）
var relicSlotIndex = map[string]int{
	"EQUIP_BRACER": 1, "生之花": 1,
	"EQUIP_NECKLACE": 2, "死之羽": 2,
	"EQUIP_SHOES": 3, "时之沙": 3,
	"EQUIP_RING": 4, "空之杯": 4,
	"EQUIP_DRESS": 5, "理之冠": 5,
}

// relicMainCandidates 各部位主词条候选（extra.js 的 mainAttr；花/羽主词条固定，不参与计分）
var relicMainCandidates = map[int][]string{
	3: {"atk", "def", "hp", "mastery", "recharge"},
	4: {"atk", "def", "hp", "mastery", "dmg", "phy"},
	5: {"atk", "def", "hp", "mastery", "heal", "cpct", "cdmg"},
}

// relicSubCandidates 副词条候选（extra.js 的 subAttr，共 10 项）
var relicSubCandidates = []string{
	"atk", "atkPlus", "def", "defPlus", "hp", "hpPlus", "mastery", "recharge", "cpct", "cdmg",
}

// relicDefaultWeight 无角色权重配置时的兜底（ArtisMarkCfg.js 的 defaultAttrWeight）
var relicDefaultWeight = map[string]int{"atk": 75, "cpct": 100, "cdmg": 100, "dmg": 100, "phy": 100}

// relicClasses 分档阈值（ArtisMark.getMarkClass：取第一个 mark < 阈值的档）
var relicClasses = []struct {
	Name  string
	Limit float64
}{
	{"D", 7}, {"C", 14}, {"B", 21}, {"A", 28}, {"S", 35},
	{"SS", 42}, {"SSS", 49}, {"ACE", 56}, {"MAX", 70},
}

// ───────────────────────── 动态规则的求值 ─────────────────────────
//
// 三类"上游动态规则"（武器适配 / 西风系列 / 绝缘 4 件）**已全部落库**
// （2026-09-27；此前硬编码在 Go 里 ⇒ 改一次要发版，与角色级规则的"改数据不发版"不一致）。
// 现在它们就是 character_relic_weight_rules 里 character_id = 0 的通用规则行，
// 差异只体现在 Effect / CondType 上：
//   delta_refine  （武器适配，按精炼线性）   condType=weapon
//   raise_to      （西风系列 → 暴击 100）    condType=weapon_regex
//   raise_to_max_of（绝缘 4 件 → 充能拉齐）  condType=set + condNum=4

// relicRuleRegexCache 缓存 weapon_regex 规则的编译结果（规则少、进程内长期复用；
// 失败也缓存为 nil，避免每次请求重复编译坏正则）
var relicRuleRegexCache sync.Map // pattern -> *regexp.Regexp（nil = 编译失败）

func relicRuleRegex(pattern string) (*regexp.Regexp, bool) {
	if v, ok := relicRuleRegexCache.Load(pattern); ok {
		re, _ := v.(*regexp.Regexp)
		return re, re != nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		relicRuleRegexCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil, false
	}
	relicRuleRegexCache.Store(pattern, re)
	return re, true
}

// ───────────────────────── 入参与出参 ─────────────────────────

// RelicStatInput 一条副词条
type RelicStatInput struct {
	Name  string  // 中文词条名（如「暴击率」）
	Value float64 // 数值；百分比词条填百分数本身（19.4 表示 19.4%）
	Count int     // 强化次数（展示用，当前不参与计分）
}

// RelicScoreInput 单件圣遗物算分入参
type RelicScoreInput struct {
	CharacterID   int64
	Slot          string // EQUIP_* 或中文部位名
	MainStatName  string // 中文主词条名（花/羽可不填）
	MainStatValue float64
	SubStats      []RelicStatInput

	// 角色上下文：用于权重覆盖与动态规则
	SetName       string // 套装名（绝缘4 规则用）
	SetCount      int    // 该套装件数（绝缘4 规则用）
	WeaponName    string
	WeaponAffix   int // 精炼 1~5，0 视为 1
	Constellation int

	// 角色基础值：全 0 时用上游默认 { hp: 14000, atk: 230, def: 700 }
	BaseHP  float64
	BaseATK float64
	BaseDEF float64
}

// RelicScoreResult 单件算分结果
type RelicScoreResult struct {
	Score     float64            // 口径 A：0~66（复刻上游）
	Percent   float64            // 口径 B：占该部位理论上限的百分比
	Class     string             // 分档
	WordCount float64            // 总词条数（全部副词条，均值口径）
	EffCount  float64            // 有效词条数（按角色权重加权，均值口径）
	CritScore float64            // 双暴分 = 暴击率 × 2 + 暴击伤害（仅副词条）
	WeightTag string             // 命中的规则标签（专武 / 高命 / 西风 / 绝缘4），多个用 | 连接
	Dims      map[string]float64 // 各维度加权贡献（排查与展示）
}

type relicAttr struct {
	weight    int
	fixWeight float64
	mark      float64
}

// ───────────────────────── 对外接口 ─────────────────────────

// ScoreRelic 计算单件圣遗物分数；false 表示无法计分（部位/主词条不可识别）
func (s *Service) ScoreRelic(in RelicScoreInput) (RelicScoreResult, bool) {
	slot, ok := relicSlotIndex[strings.TrimSpace(in.Slot)]
	if !ok {
		return RelicScoreResult{}, false
	}
	mainDim := relicDimByName[strings.TrimSpace(in.MainStatName)]

	// 2026-09-27 用户决定：**条件规则一律不参与算分** ✗
	// （西风系列 / 绝缘 4 件 / 武器适配 / 专精武 都不再改动权重）—— 直接取角色默认权重。
	// `effectiveRelicWeight` 与规则数据（character_relic_weight_rules）保留但**不再被调用** ✓（便于参考与回滚）。
	// 注：线上默认口径本就是 mp（小助手），而 mp 路径从未吃规则 ✓ —— 所以这次改动对现网分数**无影响** ✓。
	w := s.baseRelicWeight(in.CharacterID)
	tag := ""
	attrs := relicAttrs(w, relicBaseValues(in))
	posMax := relicPosMaxMark(attrs, slot)
	if posMax <= 0 {
		return RelicScoreResult{}, false
	}

	raw := 0.0
	dims := map[string]float64{}

	// 主词条（沙/杯/冠）：mark × 数值 ÷ 4（上游：主词条 ≈ 4 个词条量）
	fixPct := 1.0
	if slot >= 3 && mainDim != "" {
		if a, exist := attrs[mainDim]; exist {
			c := a.mark * in.MainStatValue / 4
			raw += c
			dims[mainDim] += c
		}
		// 主词条契合度：充能主词条不参与折算（上游 if (key !== 'recharge')）
		if mainDim != "recharge" {
			if mMax := relicPosMaxMain(attrs, slot); mMax > 0 {
				fixPct = math.Max(0, math.Min(1, float64(relicWeightOfDim(w, mainDim))/mMax))
			}
			// 攻/生/防 权重 ≥ 75 视为完全契合
			if (mainDim == "atk" || mainDim == "hp" || mainDim == "def") && relicWeightOfDim(w, mainDim) >= 75 {
				fixPct = 1
			}
		}
	}

	// 副词条：Σ mark × 数值
	wordCount, effCount, crit := 0.0, 0.0, 0.0
	for _, sub := range in.SubStats {
		dim := relicDimByName[strings.TrimSpace(sub.Name)]
		if dim == "" {
			continue
		}
		if a, exist := attrs[dim]; exist {
			c := a.mark * sub.Value
			raw += c
			dims[dim] += c
		}
		if avg := relicAvgTier[dim]; avg > 0 {
			words := sub.Value / avg
			wordCount += words
			if weight := relicWeightOfDim(w, dim); weight > 0 {
				effCount += words * float64(weight) / 100
			}
		}
		switch dim {
		case "cpct":
			crit += sub.Value * 2
		case "cdmg":
			crit += sub.Value
		}
	}

	score := raw * (1 + fixPct) / 2 / posMax * relicScoreScale
	return RelicScoreResult{
		Score:     score,
		Percent:   raw / posMax * 100,
		Class:     RelicClass(score),
		WordCount: wordCount,
		EffCount:  effCount,
		CritScore: crit,
		WeightTag: tag,
		Dims:      dims,
	}, true
}

// RelicClass 单件/平均分 → 档位（阈值同上游 getMarkClass）
func RelicClass(mark float64) string {
	for _, c := range relicClasses {
		if mark < c.Limit {
			return c.Name
		}
	}
	return relicClasses[len(relicClasses)-1].Name // ≥ 70 也归 MAX（上游返回 undefined，这里兜底）
}

// RelicSetGrade 套装评级：上游按 **5 件平均分** 分档（ArtisMark.getMarkClass(total / 5)）
func RelicSetGrade(total float64, count int) (string, float64) {
	if count <= 0 {
		return "", 0
	}
	avg := total / float64(count)
	return RelicClass(avg), avg
}

// ParseRelicStatValue 解析 dto 侧的字符串数值（"19.4%" → 19.4，"4780" → 4780）。
// dto.RelicSubStat.Value / RelicInfo.MainStat 是字符串（translate 层已格式化），
// 拿不到 float 时用这个反解；能拿到原始 float 时应优先传 float（见 ai/08 §9 备注）。
func ParseRelicStatValue(v string) float64 {
	s := strings.TrimSpace(v)
	s = strings.TrimSuffix(s, "%")
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// ParseRelicMainStat 解析主词条字符串（"生命值 +4780" → "生命值", 4780）
func ParseRelicMainStat(mainStat string) (string, float64) {
	s := strings.TrimSpace(mainStat)
	if s == "" {
		return "", 0
	}
	idx := strings.Index(s, "+")
	if idx < 0 {
		return s, 0
	}
	name := strings.TrimSpace(s[:idx])
	return name, ParseRelicStatValue(s[idx+1:])
}

// ───────────────────────── 权重求值 ─────────────────────────

// baseRelicWeight 角色的**默认权重**（不含任何条件规则）：查不到该角色就退回通用默认表 ✓
func (s *Service) baseRelicWeight(characterID int64) CharacterRelicWeight {
	w, ok := s.RelicWeight(characterID)
	if !ok {
		w = CharacterRelicWeight{CharacterID: characterID}
		for dim, v := range relicDefaultWeight {
			w.setDim(dim, v)
		}
	}
	return w
}

// ⚠️ **已停用（2026-09-27）**：条件规则不再参与算分 ✗ —— `ScoreRelic` 现在直接调 `baseRelicWeight`。
// 本函数与 `character_relic_weight_rules` 的 9 条数据都保留（便于参考 / 必要时回滚 ✓），
// 但**没有任何生产调用点**（只剩单测在守着它的行为，见 relic_score_test.go）。
//
// effectiveRelicWeight 组装该角色本次算分实际生效的权重：
//
//	默认权重（兜底） → 角色默认权重表 → 覆盖规则（DB：角色级 + 通用级，按 Priority 求值）
//
// 覆盖规则全部走数据（character_relic_weight_rules），**Go 里不再有硬编码规则**：
// 武器适配 / 西风系列 / 绝缘 4 件都是 character_id = 0 的通用行（见文件上半部分的说明）。
func (s *Service) effectiveRelicWeight(in RelicScoreInput) (CharacterRelicWeight, string) {
	w := s.baseRelicWeight(in.CharacterID)

	var tags []string
	for _, r := range s.relicRulesFor(in.CharacterID) {
		if !relicRuleHit(r, in) || !relicRuleRequireOK(r, w) {
			continue
		}
		if relicRuleApply(&w, r, in) && r.Note != "" {
			tags = append(tags, r.Note)
		}
	}
	return w, strings.Join(tags, "|")
}

// relicRulesFor 该角色本次生效的规则：角色级 + 通用级（character_id = 0），
// 按 Priority 升序（同序按 ID）—— 顺序会影响结果（先加减、后抬升），所以不能只靠 ID。
func (s *Service) relicRulesFor(characterID int64) []CharacterRelicWeightRule {
	out := append(
		append([]CharacterRelicWeightRule{}, s.RelicWeightRules(characterID)...),
		s.RelicWeightRules(0)...,
	)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// relicRuleRequireOK 前置条件：RequireStat 为空则无条件生效；
// 否则要求该维当前权重 ≥ RequireMin（上游武器适配只对攻击型角色生效 = atk ≥ 1）
func relicRuleRequireOK(r CharacterRelicWeightRule, w CharacterRelicWeight) bool {
	if r.RequireStat == "" {
		return true
	}
	return w.Dim(r.RequireStat) >= r.RequireMin
}

// relicRuleApply 应用一条规则，返回**是否真的改动了权重**。
// 与数据化之前的实现严格对齐：加法恒视为生效（角色级规则命中就打标签），
// 抬升类只在真的抬起来时才算生效（否则不重复打标签）。
func relicRuleApply(w *CharacterRelicWeight, r CharacterRelicWeightRule, in RelicScoreInput) bool {
	switch r.EffectOrDefault() {
	case RelicEffectDelta:
		w.setDim(r.StatKey, w.Dim(r.StatKey)+r.Delta)
		return true

	case RelicEffectDeltaRefine:
		cur := w.Dim(r.StatKey)
		if cur >= 100 {
			return false
		}
		affix := in.WeaponAffix
		if affix < 1 {
			affix = 1
		}
		if affix > 5 {
			affix = 5
		}
		plus := r.ValueMin + int(math.Round(float64(r.ValueMax-r.ValueMin)*float64(affix-1)/4.0))
		next := cur + plus
		if next > 100 {
			next = 100
		}
		w.setDim(r.StatKey, next)
		return true

	case RelicEffectRaiseTo:
		if w.Dim(r.StatKey) >= r.Value {
			return false
		}
		w.setDim(r.StatKey, r.Value)
		return true

	case RelicEffectRaiseToMaxOf:
		best := 0
		for _, dim := range strings.Split(r.Dims, ",") {
			dim = strings.TrimSpace(dim)
			if dim == "" || dim == r.StatKey {
				continue
			}
			if v := w.Dim(dim); v > best {
				best = v
			}
		}
		if best == 0 || w.Dim(r.StatKey) >= best {
			return false
		}
		w.setDim(r.StatKey, best)
		return true
	}
	return false
}

func relicRuleHit(r CharacterRelicWeightRule, in RelicScoreInput) bool {
	switch r.CondType {
	case "weapon":
		return r.CondValue != "" && r.CondValue == in.WeaponName
	case "weapon_regex":
		if r.CondValue == "" || in.WeaponName == "" {
			return false
		}
		re, ok := relicRuleRegex(r.CondValue)
		return ok && re.MatchString(in.WeaponName)
	case "set":
		if r.CondValue == "" || r.CondValue != in.SetName {
			return false
		}
		return relicCompareNum(in.SetCount, r.CondOp, r.CondNum)
	case "cons":
		n, err := strconv.Atoi(strings.TrimSpace(r.CondValue))
		if err != nil {
			return false
		}
		return relicCompareNum(in.Constellation, r.CondOp, n)
	}
	return false
}

// relicCompareNum 数值比较（CondOp 缺省按等号；> / < 供将来扩展）
func relicCompareNum(v int, op string, target int) bool {
	switch op {
	case ">=":
		return v >= target
	case "<=":
		return v <= target
	case ">":
		return v > target
	case "<":
		return v < target
	default:
		return v == target
	}
}

// (w *CharacterRelicWeight) setDim 自增/覆盖某维度权重（白值维度归到其 base 维度）
func (w *CharacterRelicWeight) setDim(key string, v int) {
	switch key {
	case "hp":
		w.HP = v
	case "atk":
		w.ATK = v
	case "def":
		w.DEF = v
	case "cpct":
		w.CPct = v
	case "cdmg":
		w.CDmg = v
	case "mastery":
		w.Mastery = v
	case "dmg":
		w.Dmg = v
	case "phy":
		w.Phy = v
	case "recharge":
		w.Recharge = v
	case "heal":
		w.Heal = v
	case "hpPlus":
		w.HP = v
	case "atkPlus":
		w.ATK = v
	case "defPlus":
		w.DEF = v
	}
}

// relicWeightOfDim 取维度权重：白值维度取其 base 维度的权重（上游 attrWeight[k || key]）
func relicWeightOfDim(w CharacterRelicWeight, dim string) int {
	if base, ok := relicBaseOf[dim]; ok {
		return w.Dim(base)
	}
	return w.Dim(dim)
}

// ───────────────────────── 内部计算 ─────────────────────────

func relicBaseValues(in RelicScoreInput) map[string]float64 {
	base := map[string]float64{
		"hp":  relicDefaultBaseHP,
		"atk": relicDefaultBaseATK,
		"def": relicDefaultBaseDEF,
	}
	if in.BaseHP > 0 {
		base["hp"] = in.BaseHP
	}
	if in.BaseATK > 0 {
		base["atk"] = in.BaseATK
	}
	if in.BaseDEF > 0 {
		base["def"] = in.BaseDEF
	}
	return base
}

// relicAttrs 逐维度算 mark / fixWeight（复刻 ArtisMarkCfg.getCfg）
//
//	无 base：mark = weight / 最大档；        fixWeight = weight
//	有 base：mark = weight / base档 / (基础值+plus) × 100
//	         fixWeight = weight × 本档 / base档 / (基础值+plus) × 100
//	         —— 这把「白值词条」折算成与百分比词条可比的等效权重
func relicAttrs(w CharacterRelicWeight, base map[string]float64) map[string]relicAttr {
	out := make(map[string]relicAttr, len(relicAllDims))
	for _, dim := range relicAllDims {
		weight := relicWeightOfDim(w, dim)
		if weight <= 0 {
			continue // 权 0 → 该维度不计分（上游行为）
		}
		tier, ok := relicMaxTier[dim]
		if !ok || tier <= 0 {
			continue
		}
		if b, hasBase := relicBaseOf[dim]; hasBase {
			bv := base[b]
			if bv <= 0 {
				bv = 1
			}
			plus := 0.0
			if b == "atk" {
				plus = relicAtkBasePlus
			}
			bt := relicMaxTier[b]
			if bt <= 0 {
				continue
			}
			out[dim] = relicAttr{
				weight:    weight,
				mark:      float64(weight) / bt / (bv + plus) * 100,
				fixWeight: float64(weight) * tier / bt / (bv + plus) * 100,
			}
			continue
		}
		out[dim] = relicAttr{
			weight:    weight,
			mark:      float64(weight) / tier,
			fixWeight: float64(weight),
		}
	}
	return out
}

// relicPosMaxMain 该部位主词条候选里最高的 fixWeight（对应上游 posMaxMark['m'+idx]）
func relicPosMaxMain(attrs map[string]relicAttr, slot int) float64 {
	best := 0.0
	for _, dim := range relicMainCandidates[slot] {
		if a, ok := attrs[dim]; ok && a.fixWeight > best {
			best = a.fixWeight
		}
	}
	return best
}

// relicPosMaxMark 该部位"理论上限"（复刻 ArtisMark.getMaxMark）：
//
//	· 沙/杯/冠：主词条取最优候选 × 2；副词条取最优 4 条、**首条 ×6**、其余 ×1（= 初始 4 词条 + 5 次强化）
//	· 花/羽：只有副词条，同样取最优 4 条、首条 ×6
func relicPosMaxMark(attrs map[string]relicAttr, slot int) float64 {
	total := 0.0
	banned := ""
	if slot >= 3 {
		bestDim, bestMark := "", 0.0
		for _, dim := range relicMainCandidates[slot] {
			if a, ok := attrs[dim]; ok && a.fixWeight > bestMark {
				bestDim, bestMark = dim, a.fixWeight
			}
		}
		if bestDim != "" {
			total += bestMark * 2
			banned = bestDim
		}
	}

	type cand struct {
		dim   string
		value float64
	}
	list := make([]cand, 0, len(relicSubCandidates))
	for _, dim := range relicSubCandidates {
		if dim == banned {
			continue
		}
		if a, ok := attrs[dim]; ok {
			list = append(list, cand{dim: dim, value: a.fixWeight})
		}
	}
	// 稳定排序：值降序，同值按维度名升序（上游用 lodash.sortBy+reverse，这里保证可复现）
	sort.Slice(list, func(i, j int) bool {
		if list[i].value != list[j].value {
			return list[i].value > list[j].value
		}
		return list[i].dim < list[j].dim
	})
	for i, c := range list {
		if i >= 4 {
			break
		}
		if i == 0 {
			total += c.value * 6
			continue
		}
		total += c.value
	}
	return total
}
