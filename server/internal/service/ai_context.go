package service

// 玩家上下文组装（对接契约见 03-主服务对接说明 §6）
//
// 分级注入策略：
//   - 所有展柜角色都给「概览」：名字 / 等级 / 命座 / 天赋 / 武器
//   - 只有「提问中点名的角色」或「养成类意图」才展开圣遗物明细（含主词条）
//   - 超出预算时整条丢弃末尾角色，并在 summary 注明省略数量（不给半个角色条目）
//
// 约定：数据只在本次请求内存中存在，AI 服务不落库。

import (
	"strconv"
	"strings"

	"teyvat-guide/server/internal/dto"
)

const (
	ctxTokenBudget = 1500 // 上下文 token 预算（中文按 1 字符 ≈ 1 token 粗估）
)

// 意图关键词（按优先级从上到下匹配）
var goalKeywords = []struct {
	goal string
	keys []string
}{
	{"team_build", []string{"配队", "队伍", "阵容", "双核", "三保一", "辅助谁", "队友"}},
	{"character_build", []string{"圣遗物", "词条", "副词条", "面板", "武器", "命座", "天赋", "养成", "培养", "练", "堆", "毕业", "圣遗", "build", "artifact"}},
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

// isBuildingIntent 是否为养成 / 配队类意图（需要圣遗物明细）
func isBuildingIntent(goal string) bool {
	return goal == "character_build" || goal == "team_build"
}

// BuildPlayerContext 组装玩家上下文；profile 为空或展柜为空时返回 nil（不推送）
func BuildPlayerContext(profile *dto.PlayerProfile, question string) *RequestContext {
	if profile == nil || len(profile.Characters) == 0 {
		return nil
	}

	goal := detectGoal(question)
	needDetail := isBuildingIntent(goal)
	mentioned := mentionedNames(question, profile.Characters)

	chars := make([]map[string]any, 0, len(profile.Characters))
	used := 0
	omitted := 0
	for _, c := range profile.Characters {
		// 点名角色或养成/配队意图 → 展开圣遗物明细
		detail := needDetail || mentioned[c.Name]
		entry := characterEntry(c, detail)

		cost := roughTokens(entry)
		if used+cost > ctxTokenBudget && len(chars) > 0 {
			omitted = len(profile.Characters) - len(chars)
			break // 整条丢弃，避免出现半个角色条目
		}
		used += cost
		chars = append(chars, entry)
	}

	player := map[string]any{
		"uid":        profile.UID,
		"nickname":   profile.Nickname,
		"level":      profile.Level,
		"worldLevel": profile.WorldLevel,
		"characters": chars,
	}
	rc := &RequestContext{Goal: goal, Player: player}
	if omitted > 0 {
		rc.Summary = "其余 " + strconv.Itoa(omitted) + " 个展柜角色因体积预算已省略"
	}
	return rc
}

// mentionedNames 找出提问中被点名的角色名
func mentionedNames(question string, chars []dto.CharacterCard) map[string]bool {
	q := strings.ToLower(question)
	out := make(map[string]bool, 4)
	for _, c := range chars {
		if c.Name != "" && strings.Contains(q, strings.ToLower(c.Name)) {
			out[c.Name] = true
		}
	}
	return out
}

// characterEntry 单个角色的上下文条目
func characterEntry(c dto.CharacterCard, detail bool) map[string]any {
	entry := map[string]any{
		"name":          c.Name,
		"level":         c.Level,
		"constellation": c.Constellation,
	}
	if c.Talents != nil {
		entry["talents"] = map[string]int{
			"normal": c.Talents.Normal,
			"skill":  c.Talents.Skill,
			"burst":  c.Talents.Burst,
		}
	}
	if c.Weapon != nil {
		entry["weapon"] = map[string]any{
			"name":   c.Weapon.Name,
			"refine": c.Weapon.Affix,
			"level":  c.Weapon.Level,
		}
	}
	if arts := artifactPayload(c.Relics, detail); len(arts) > 0 {
		entry["artifacts"] = arts
	}
	return entry
}

// artifactPayload 圣遗物按套装聚合：
//   - 概览：[{set, count}]
//   - 明细：[{set, count, mainStats:{部位: 词条名}}]
func artifactPayload(relics []dto.RelicInfo, detail bool) []map[string]any {
	if len(relics) == 0 {
		return nil
	}
	type agg struct {
		count int
		mains map[string]string
	}
	order := make([]string, 0, len(relics))
	grouped := make(map[string]*agg, len(relics))

	for _, r := range relics {
		set := firstNonEmpty(r.Set, r.Name)
		if set == "" {
			continue
		}
		g, ok := grouped[set]
		if !ok {
			g = &agg{mains: map[string]string{}}
			grouped[set] = g
			order = append(order, set)
		}
		g.count++
		if detail {
			slot := firstNonEmpty(r.Slot, r.Pos)
			if stat := statNameOnly(r.MainStat); stat != "" {
				g.mains[slot] = stat
			}
		}
	}

	out := make([]map[string]any, 0, len(order))
	for _, set := range order {
		g := grouped[set]
		item := map[string]any{"set": set, "count": g.count}
		if detail && len(g.mains) > 0 {
			item["mainStats"] = g.mains
		}
		out = append(out, item)
	}
	return out
}

// statNameOnly 从 "生命值 +4780" 提取词条名（丢掉数值，契约只要名）
func statNameOnly(mainStat string) string {
	if mainStat == "" {
		return ""
	}
	if idx := strings.Index(mainStat, " +"); idx > 0 {
		return mainStat[:idx]
	}
	return mainStat
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
	case []map[string]any:
		n := 0
		for _, item := range t {
			n += roughTokens(item)
		}
		return n
	case map[string]int:
		n := 0
		for k, v := range t {
			n += len([]rune(k)) + len(strconv.Itoa(v))
		}
		return n
	case map[string]string:
		n := 0
		for k, v := range t {
			n += len([]rune(k)) + len([]rune(v))
		}
		return n
	case int:
		return len(strconv.Itoa(t))
	default:
		return 8
	}
}
