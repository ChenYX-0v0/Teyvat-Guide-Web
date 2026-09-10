package service

// 玩家上下文组装（P1 §6）：
// 主服务在调用 AI 前，把玩家档案压缩成 context 随请求下发，
// 让「养成分析」类问题能基于用户真实角色池作答。
//
// 约定（与 AI 服务）：
//   - 总量 ≤ 1500 tokens（AI 服务还会二次裁剪）
//   - 只推送「与提问相关」的角色的摘要，禁止整份档案/全图鉴
//   - 用户私有数据只在本次请求内存中存在，AI 服务不落库

import (
	"strings"

	"teyvat-guide/server/internal/dto"
)

const (
	ctxTokenBudget = 1500 // 上下文 token 预算（中文按 1 字符 ≈ 1 token 粗估）
	maxDetailed    = 2    // 命中提问的角色：给详细面板
	maxBrief       = 6    // 其余相关角色：只给摘要
)

// 意图关键词（按优先级从上到下匹配）
var goalKeywords = []struct {
	goal string
	keys []string
}{
	{"team_build", []string{"配队", "队伍", "阵容", "双核", "三保一", "辅助谁", "队友"}},
	{"character_build", []string{"圣遗物", "词条", "副词条", "面板", "武器", "命座", "天赋", "养成", "培养", "练", "堆", "毕业", "毕业面板", "圣遗", "build", "artifact"}},
	{"exploration", []string{"探索", "宝箱", "锚点", "地图", "采集", "材料在哪", "路线"}},
}

func detectGoal(question string) string {
	q := strings.ToLower(question)
	for _, g := range goalKeywords {
		for _, k := range g.keys {
			if strings.Contains(q, strings.ToLower(k)) {
				return g.goal
			}
		}
	}
	return "general"
}

// BuildPlayerContext 组装玩家上下文；profile 为空或展柜为空时返回 nil（不推送）
func BuildPlayerContext(profile *dto.PlayerProfile, question string) *RequestContext {
	if profile == nil || len(profile.Characters) == 0 {
		return nil
	}

	goal := detectGoal(question)
	detailed, brief := selectCharacters(profile.Characters, question, goal)

	player := map[string]any{
		"uid":        profile.UID,
		"level":      profile.Level,
		"worldLevel": profile.WorldLevel,
		"note":       "以上为本账号展柜角色的真实数据，回答养成问题时请结合具体面板与词条，不要泛泛而谈",
	}
	// characters：与提问相关的角色详情（对接文档约定的主字段名）
	if d := buildDetailed(detailed, ctxTokenBudget); len(d) > 0 {
		player["characters"] = d
	}
	// otherCharacters：展柜中其余角色的精简摘要，供配队类问题参考
	if b := buildBrief(brief); len(b) > 0 {
		player["otherCharacters"] = b
	}
	return &RequestContext{Goal: goal, Player: player}
}

// selectCharacters 选出与提问最相关的角色：命中名字的给详细，其余按等级给摘要
func selectCharacters(chars []dto.CharacterCard, question string, goal string) (detailed, brief []dto.CharacterCard) {
	q := strings.ToLower(question)
	matched := make([]dto.CharacterCard, 0, 2)
	rest := make([]dto.CharacterCard, 0, len(chars))

	for _, c := range chars {
		if c.Name != "" && strings.Contains(q, strings.ToLower(c.Name)) {
			matched = append(matched, c)
		} else {
			rest = append(rest, c)
		}
	}

	// 按等级降序（展柜顺序通常已按玩家习惯，等级高者更可能是主力）
	for i := 1; i < len(rest); i++ {
		for j := i; j > 0 && rest[j].Level > rest[j-1].Level; j-- {
			rest[j], rest[j-1] = rest[j-1], rest[j]
		}
	}

	detailed = matched
	if len(detailed) > maxDetailed {
		detailed = detailed[:maxDetailed]
	}

	// 养成 / 配队类问题才附带其余角色摘要，闲聊不推送
	if goal == "character_build" || goal == "team_build" {
		limit := maxBrief
		if len(rest) < limit {
			limit = len(rest)
		}
		brief = rest[:limit]
	}
	return detailed, brief
}

// buildDetailed 详细条目：基础信息 + 面板 + 主词条 + 推荐副词条
func buildDetailed(chars []dto.CharacterCard, budget int) []map[string]any {
	out := make([]map[string]any, 0, len(chars))
	used := 0
	for _, c := range chars {
		entry := map[string]any{
			"name":          c.Name,
			"element":       c.Element,
			"level":         c.Level,
			"constellation": c.Constellation,
			"fetter":        c.Fetter,
		}
		if c.Weapon != nil {
			// AI 服务 schema 要求 weapon 为对象
			entry["weapon"] = map[string]any{
				"name":       c.Weapon.Name,
				"rarity":     c.Weapon.Rarity,
				"level":      c.Weapon.Level,
				"refinement": c.Weapon.Affix,
			}
		}
		if len(c.Stats) > 0 {
			panel := make([]string, 0, len(c.Stats))
			for _, s := range c.Stats {
				panel = append(panel, s.Name+" "+s.Value)
			}
			entry["panel"] = panel
		}
		if mains := relicMainStats(c.Relics); len(mains) > 0 {
			entry["relicMainStats"] = mains
		}
		if c.StatWeights != nil && len(c.StatWeights.SubStats) > 0 {
			rec := make([]string, 0, 3)
			for _, s := range c.StatWeights.SubStats {
				rec = append(rec, s.Name)
				if len(rec) >= 3 {
					break
				}
			}
			entry["recommendedSubStats"] = rec
		}

		cost := roughTokens(entry)
		if used+cost > budget {
			break
		}
		used += cost
		out = append(out, entry)
	}
	return out
}

// buildBrief 摘要条目：名字 / 元素 / 等级 / 命座
func buildBrief(chars []dto.CharacterCard) []map[string]any {
	out := make([]map[string]any, 0, len(chars))
	for _, c := range chars {
		out = append(out, map[string]any{
			"name":          c.Name,
			"element":       c.Element,
			"level":         c.Level,
			"constellation": c.Constellation,
		})
	}
	return out
}

func relicMainStats(relics []dto.RelicInfo) []string {
	if len(relics) == 0 {
		return nil
	}
	out := make([]string, 0, len(relics))
	for _, r := range relics {
		if r.MainStat != "" {
			out = append(out, r.MainStat)
		}
	}
	return out
}

// roughTokens 粗略估算条目体积（中文 1 字符 ≈ 1 token）
func roughTokens(v any) int {
	switch t := v.(type) {
	case string:
		return len([]rune(t))
	case []string:
		n := 0
		for _, s := range t {
			n += len([]rune(s))
		}
		return n
	case map[string]any:
		n := 0
		for k, val := range t {
			n += len([]rune(k)) + roughTokens(val)
		}
		return n
	default:
		return 8
	}
}
