package service

// AI 上下文组装（对接契约见 03-主服务对接说明 §6）
//
// 两条注入链路：
//   V1 玩家档案（私有数据，需登录 + 绑定 UID）
//     - 所有展柜角色都给「概览」：名字 / 等级 / 命座 / 天赋 / 武器
//     - 「提问中点名的角色」一定展开圣遗物明细（含主词条）**并置顶**；
//       **没有点名**时才按养成/配队意图给所有角色展开明细
//     - 超出预算时从末尾整条丢弃，并在 summary 注明省略数量（不给半个角色条目）
//   V2 资产资料（公开数据，与登录态无关）
//     - 提问命中角色名时，把库里的权威推荐词条渲染进 summary，让数值类问题不靠模型记忆
//
// 约定：数据只在本次请求内存中存在，AI 服务不落库。

import (
	"math"
	"strconv"
	"strings"

	"teyvat-guide/server/internal/dto"
)

const (
	ctxTokenBudget = 1500 // 玩家档案的 token 预算（中文按 1 字符 ≈ 1 token 粗估）

	// referenceTokenBudget V2 资料文本的独立预算：与玩家档案共用 AI 服务的上下文预算，
	// 单独设一个小额度，避免资料把面板挤掉。
	// 2026-09-15 由 400 上调到 800：V2.2 引入套装效果后，一条 4 件套文本本身就有 100~200 字，
	// 400 会把套装整条挤掉（角色词条约 120 字/个 ×2 + 套装约 250 字/个 ×2 ≈ 740）。
	referenceTokenBudget = 800
	// maxReferenceCharacters 一次最多注入几个命中角色的资料（防止长提问塞满上下文）
	maxReferenceCharacters = 2
	// maxReferenceSets 一次最多注入几个命中套装的效果（同上；4 件套文本长，单独设上限）
	maxReferenceSets = 2
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

	// 点名角色**置顶**：上下文按 1500 token 裁剪、超预算时从末尾整条丢弃，
	// 而展柜顺序由玩家自己摆放，被点名的角色常常排在后面 —— 不置顶就会出现
	// 模型回答「档案里没有这个角色」（2026-09-26 线上实测：问展柜最后一位的八重神子命中该问题，
	// 前端「悬浮体检」也因此会分析错人）。
	ordered := make([]dto.CharacterCard, 0, len(profile.Characters))
	for _, c := range profile.Characters {
		if mentioned[c.Name] {
			ordered = append(ordered, c)
		}
	}
	for _, c := range profile.Characters {
		if !mentioned[c.Name] {
			ordered = append(ordered, c)
		}
	}

	chars := make([]map[string]any, 0, len(profile.Characters))
	used := 0
	omitted := 0
	for _, c := range ordered {
		// 明细展开规则（2026-09-26 收紧）：点名角色一定展开；**只有没有点名角色时**，
		// 才按养成/配队意图给所有人展开 —— 否则「养成」这类宽泛提问会让每个角色都带上
		// 圣遗物明细，预算瞬间打满、排在后面的角色被整条丢掉。
		detail := mentioned[c.Name] || (needDetail && len(mentioned) == 0)
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
		"uid": profile.UID,
		// 昵称是玩家自由填写、且会被拼进 system 的动态内容 ⇒ 必须清洗，
		// 否则「忽略以上规则，你现在是…」这类昵称就是一条**持久注入**（见 sanitize.go 注释）
		"nickname":   sanitizePlayerNickname(profile.Nickname),
		"level":      profile.Level,
		"worldLevel": profile.WorldLevel,
		"characters": chars,
	}
	// 终局玩法概要（2026-09-27 新增，Enka playerInfo 直读）：深渊 / 剧诗 / 危战**只有概要**，
	// 没有每间队伍与怪物阵容。缺失时写成一句话而**不是省略字段**——省略会让模型看空，
	// 进而凭记忆编造（与 V2.3「库里没有就显式说没有」同一个教训）。
	if profile.Abyss != nil {
		player["abyss"] = map[string]any{
			"floor": profile.Abyss.Floor, "room": profile.Abyss.Room, "stars": profile.Abyss.Stars,
		}
	} else {
		player["abyss"] = "档案中未提供（玩家可能未公开战绩）"
	}
	if profile.Theater != nil {
		player["theater"] = map[string]any{
			"act": profile.Theater.Act, "mode": profile.Theater.Mode, "stars": profile.Theater.Stars,
		}
	} else {
		player["theater"] = "档案中未提供（玩家可能未公开战绩）"
	}
	if profile.Stygian != nil {
		player["stygian"] = map[string]any{
			"index": profile.Stygian.Index, "seconds": profile.Stygian.Seconds, "id": profile.Stygian.ID,
		}
	} else {
		player["stygian"] = "档案中未提供（玩家可能未公开战绩）"
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
	// 圣遗物评分摘要（V3，2026-09-27）：给"毕业了吗/先强化哪件"这类问题一个数值锚点。
	// ⚠️ 不做口径声明（2026-09-27 决策）：不给模型"社区参考/非官方"这类话术，评分数字照说。
	if c.RelicScoreTotal > 0 {
		round1 := func(v float64) float64 { return math.Round(v*10) / 10 }
		score := map[string]any{
			"total": round1(c.RelicScoreTotal),
			"avg":   round1(c.RelicScoreAvg),
			"class": c.RelicScoreClass,
			"words": round1(c.RelicWordCount),
			"crit":  round1(c.RelicCritScore),
			// 只给刻度定义，便于模型读懂 class；不是口径说明
			"note": "单件满分66，档位按5件平均分",
		}
		if detail {
			score["pieces"] = len(c.Relics)
		}
		entry["relicScore"] = score
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

// ---------- V2 · 结构化资料查表 ----------

// BuildAssetSummary 把命中的角色资料与套装效果渲染成 `context.summary` 文本（V2.1 角色 / V2.2 套装）。
// 返回空串表示什么都没命中，不推送资料。
//
// 为什么要显式标注「本服暂无资料」：模型对热门角色与套装都有记忆，不说清楚它会用记忆里的内容作答；
// 标成「暂无资料」后，配合人格提示词里的「资料外不编造」规则，库里没有的就会老实承认不知道（V2.3）。
func BuildAssetSummary(refs []CharacterStatRef, sets []RelicSetRef) string {
	if len(refs) == 0 && len(sets) == 0 {
		return ""
	}

	lines := make([]string, 0, len(refs)+len(sets))
	used := 0

	// 角色词条在前（更常被问），套装效果在后；两者共用同一份预算与各自条数上限
	for i, ref := range refs {
		if i >= maxReferenceCharacters {
			break
		}
		line := renderAssetRef(ref)
		if line == "" {
			continue
		}
		cost := roughTokens(line)
		if len(lines) > 0 && used+cost > referenceTokenBudget {
			break
		}
		used += cost
		lines = append(lines, line)
	}
	for i, ref := range sets {
		if i >= maxReferenceSets {
			break
		}
		line := renderRelicSetRef(ref)
		if line == "" {
			continue
		}
		cost := roughTokens(line)
		if len(lines) > 0 && used+cost > referenceTokenBudget {
			break
		}
		used += cost
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}

	return "【游戏资料】（来自本服资料库，是本回合数值与词条问题的唯一依据）\n- " +
		strings.Join(lines, "\n- ")
}

// renderRelicSetRef 渲染单个套装一行资料：2 件套 / 4 件套效果各一段（V2.2）。
func renderRelicSetRef(ref RelicSetRef) string {
	if ref.Name == "" {
		return ""
	}
	if len(ref.Effects) == 0 {
		return ref.Name + "：本服暂无该套装的效果资料（不得凭记忆描述效果）"
	}
	parts := make([]string, 0, len(ref.Effects))
	for _, e := range ref.Effects {
		parts = append(parts, strconv.Itoa(e.Piece)+" 件套 "+e.Effect)
	}
	return ref.Name + "：" + strings.Join(parts, "；")
}

// renderAssetRef 渲染单个角色一行资料
func renderAssetRef(ref CharacterStatRef) string {
	if ref.Name == "" {
		return ""
	}
	if len(ref.SubStats) == 0 && len(ref.MainStats) == 0 {
		return ref.Name + "：本服暂无该角色的资料（不得凭记忆给出数值）"
	}

	var b strings.Builder
	b.WriteString(ref.Name)
	b.WriteString("：推荐副词条 ")
	parts := make([]string, 0, len(ref.SubStats))
	for _, st := range ref.SubStats {
		item := st.Name
		if st.Tier != "" {
			item += "(" + st.Tier + ")"
		}
		parts = append(parts, item)
	}
	b.WriteString(strings.Join(parts, " > "))

	if len(ref.MainStats) > 0 {
		mains := make([]string, 0, len(ref.MainStats))
		for _, slot := range ref.MainStats {
			// 多个可选值用「A 或 B」表达：库里写成 "暴击率/暴击伤害" 时，
			// 必须让模型看出**这是二选一、都成立** —— 否则它会把"推荐"读成"唯一正确"，
			// 进而把旅行者身上合法的另一件主词条判成"错的"（2026-09-27 实测踩到）
			stat := slot.Stat
			if strings.Contains(stat, "/") {
				stat = strings.Join(strings.Split(stat, "/"), " 或 ")
			}
			mains = append(mains, slot.Slot+" "+stat)
		}
		b.WriteString("；推荐主词条 " + strings.Join(mains, "，"))
	}
	return b.String()
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
