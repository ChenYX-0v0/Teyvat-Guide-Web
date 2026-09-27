package asset

// 小助手口径（提瓦特小助手小程序；2026-09-27 用户决定全量切换）
//
// 公式来源：小程序「圣遗物评分查询」公式表（用户提供截图），逐条落成下表：
//
//	评分 = 词条数值 × 系数 × 收益权重（0~100 ⇒ /100）
//	  暴击率 ×2     暴击伤害 ×2（用户确认：按图上 ×2 算）
//	  元素精通 ×0.33
//	  元素充能效率 ×1.1979
//	  大攻击 ×1.33  大生命 ×1.33  大防御 ×1.06
//	  小攻击 ×0.398×0.5   小生命 ×0.026×0.66   小防御 ×0.335×0.66
//
// 与 miao 口径的差异（不改代码就不会混）：
//   - **不做归一化**：单件分 = 上式之和（miao 是 raw × (1+fixPct)/2 ÷ 部位上限 × 66）
//   - **主词条只算暴击率 / 暴击伤害**（各 +20 分），其它主词条不计（用户决定：先不算）；
//     miao 是"主词条按 4 个词条量折算 + 契合度修正"
//   - 白值用**固定系数**折算（miao 是按该角色基础值归一）
//   - 只有 **7 维**（生命/攻击/防御/暴击/暴伤/精通/充能）—— 增伤 / 物伤 / 治疗**不计分** ✗
//   - **档位阈值沿用既有那套**（用户决定：先不动，所以分档函数复用 RelicClass）
//
// 旅行者的七元素特殊处理：小助手给的是「旅行者」+ 7 个元素形态，
// 我们库里只有 旅行者(10000005/10000007) 与 奇偶·男性/女性(10000117/10000118)，
// 所以按 (角色ID, 元素) 取权重：有元素形态用元素形态，取不到就退回通用那一行。

import "strings"

// 小助手系数表：dim key → 系数（dim 与 miao 口径共用一套 key）
var mpCoef = map[string]float64{
	"cpct":     2.0,
	"cdmg":     2.0, // 用户 2026-09-27 确认：按公式图上的 ×2
	"mastery":  0.33,
	"recharge": 1.1979,
	"atk":      1.33, // 大攻击（百分比）
	"hp":       1.33, // 大生命
	"def":      1.06, // 大防御
	"atkPlus":  0.398 * 0.5,
	"hpPlus":   0.026 * 0.66,
	"defPlus":  0.335 * 0.66,
}

// mpMainBonus 主词条固定加分：只有暴击率 / 暴击伤害主词条 +20（用户决定：其它主词条先不算）
const mpMainBonus = 20.0

// mpMainBonusDims 能拿 +20 的主词条维度
var mpMainBonusDims = map[string]bool{"cpct": true, "cdmg": true}

// CharacterRelicWeightMP 小助手口径的收益权重（表名 character_relic_weights_mp）
//
// 7 维、0~100、0 = 该词条无效。主键是 (character_id, element)：
// 非旅行者 element = ”；旅行者按元素存多行（风/岩/雷/草/水/火/冰），
// 取不到元素形态时退回 element = ” 的通用行。
type CharacterRelicWeightMP struct {
	CharacterID int64  `gorm:"primaryKey;autoIncrement:false" json:"characterId"`
	Element     string `gorm:"primaryKey;size:8;not null;default:''" json:"element,omitempty"`
	Name        string `gorm:"size:32" json:"name,omitempty"`
	HP          int    `gorm:"not null;default:0" json:"hp"`
	ATK         int    `gorm:"not null;default:0" json:"atk"`
	DEF         int    `gorm:"not null;default:0" json:"def"`
	CPCT        int    `gorm:"column:cpct;not null;default:0" json:"cpct"`
	CDMG        int    `gorm:"column:cdmg;not null;default:0" json:"cdmg"`
	Mastery     int    `gorm:"not null;default:0" json:"mastery"`
	Recharge    int    `gorm:"not null;default:0" json:"recharge"`
	Source      string `gorm:"size:64" json:"source,omitempty"`
	SourceRef   string `gorm:"size:96" json:"sourceRef,omitempty"`
	FetchedAt   string `gorm:"size:16" json:"fetchedAt,omitempty"`
}

// MPRelicScore 小助手口径的单件结果
type MPRelicScore struct {
	Score     float64            // 单件分（未做归一化）
	Dims      map[string]float64 // 各维度贡献（调试与前端提示用）
	WordCount float64            // 词条数：按均词条值折算（沿用既有展示口径）
	CritScore float64            // 双暴分：暴击率×2 + 暴击伤害（只看副词条）
}

// mpscore 小助手口径的单件计分（权重的 0~100 由调用方传入 dims 决定计分范围）
func mpscore(in RelicScoreInput, weight CharacterRelicWeightMP) (MPRelicScore, bool) {
	res := MPRelicScore{Dims: map[string]float64{}}
	if len(in.SubStats) == 0 && in.MainStatName == "" {
		return res, false
	}

	for _, sub := range in.SubStats {
		dim, ok := relicDimByName[sub.Name]
		if !ok {
			continue
		}
		coef, ok := mpCoef[dim]
		if !ok {
			continue // 增伤 / 物伤 / 治疗等：小助手口径不参与计分
		}
		w := float64(weight.DimMP(baseDimOf(dim))) // 0 ⇒ 该词条对此角色无效
		if w <= 0 {
			continue
		}
		c := sub.Value * coef * w / 100.0
		res.Score += c
		res.Dims[dim] += c

		// 词条数与双暴分：沿用既有展示口径（与 miao 口径同源，便于横向比较）
		if avg, ok := relicAvgTier[dim]; ok && avg > 0 {
			res.WordCount += sub.Value / avg
		}
		switch dim {
		case "cpct":
			res.CritScore += sub.Value * 2
		case "cdmg":
			res.CritScore += sub.Value
		}
	}

	// 主词条：只认暴击率 / 暴击伤害，各 +20（花/羽主词条固定，不计）
	// 名字容忍 "暴击伤害" 与 "暴击伤害 +62.2%" 两种形式（翻译层给的是纯名，这里做防御）
	if relicSlotIndex[in.Slot] >= 3 && in.MainStatName != "" {
		mainName := in.MainStatName
		if i := strings.Index(mainName, " +"); i > 0 {
			mainName = mainName[:i]
		}
		if dim, ok := relicDimByName[mainName]; ok && mpMainBonusDims[dim] {
			res.Score += mpMainBonus
			res.Dims[dim] += mpMainBonus
		}
	}
	return res, true
}

// baseDimOf 白值维度（atkPlus 等）归到其 base 维度取权重
func baseDimOf(dim string) string {
	if strings.HasSuffix(dim, "Plus") {
		return strings.TrimSuffix(dim, "Plus")
	}
	return dim
}

// DimMP 小助手口径的 7 维权重取值（只有这 7 维有数据；其余一律 0 = 不计分）
func (w CharacterRelicWeightMP) DimMP(key string) int {
	switch key {
	case "hp":
		return w.HP
	case "atk":
		return w.ATK
	case "def":
		return w.DEF
	case "cpct":
		return w.CPCT
	case "cdmg":
		return w.CDMG
	case "mastery":
		return w.Mastery
	case "recharge":
		return w.Recharge
	}
	return 0
}
