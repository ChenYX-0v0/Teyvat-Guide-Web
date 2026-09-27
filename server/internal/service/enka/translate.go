package enka

// 翻译层：Enka 原始数据 → 人类可读的 dto
// 名称依赖 AssetService（MySQL 映射），未命中走 fallback 并收集缺漏

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/service/asset"
)

const enkaUIBase = "https://enka.network/ui/"

type Translator struct {
	assets *asset.Service
}

func NewTranslator(assets *asset.Service) *Translator {
	return &Translator{assets: assets}
}

// TranslateStatRecommendation 把库里的推荐词条翻译成中文词条名并按权重降序。
// 玩家档案翻译与 V2 资料查表（AI 上下文）共用这一份实现，避免两处逻辑漂移。
// 库里既无副词条也无主词条时返回 nil，供调用方区分「没有资料」。
func (tr *Translator) TranslateStatRecommendation(rec asset.StatRecommendation) *dto.StatRecommendation {
	if len(rec.SubStats) == 0 && len(rec.MainStats) == 0 {
		return nil
	}
	entries := make([]dto.StatWeightEntry, 0, len(rec.SubStats))
	for _, it := range rec.SubStats {
		name := it.StatKey
		if p, ok := tr.assets.Prop(it.StatKey); ok {
			name = p.Name
		}
		entries = append(entries, dto.StatWeightEntry{Name: name, Weight: it.Weight, Tier: it.Tier})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Weight > entries[j].Weight })
	return &dto.StatRecommendation{SubStats: entries, MainStats: rec.MainStats}
}

// deref 指针取值；nil → 0（这三个字段是"同生共死"的一组，任一枚非空即算有数据）
func deref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// abyssSummary 深境螺旋概要：**以 towerFloorIndex 是否存在**为"有没有数据"的判据；
// 玩家隐藏战绩时字段整体缺失 → 返回 nil（前端与 AI 上下文据此区分"0 层"与"没有"）。
func abyssSummary(pi PlayerInfo) *dto.AbyssSummary {
	if pi.TowerFloorIndex == nil {
		return nil
	}
	return &dto.AbyssSummary{
		Floor: deref(pi.TowerFloorIndex),
		Room:  deref(pi.TowerLevelIndex),
		Stars: deref(pi.TowerStarIndex),
	}
}

// theaterSummary 幻想真境剧诗概要（判据同上）
func theaterSummary(pi PlayerInfo) *dto.TheaterSummary {
	if pi.TheaterActIndex == nil {
		return nil
	}
	return &dto.TheaterSummary{
		Act:   deref(pi.TheaterActIndex),
		Mode:  deref(pi.TheaterModeIndex),
		Stars: deref(pi.TheaterStarIndex),
	}
}

// stygianSummary 幽境危战概要（判据同上）
func stygianSummary(pi PlayerInfo) *dto.StygianSummary {
	if pi.StygianIndex == nil {
		return nil
	}
	return &dto.StygianSummary{
		Index:   deref(pi.StygianIndex),
		Seconds: deref(pi.StygianSeconds),
		ID:      deref(pi.StygianID),
	}
}

// TranslateProfile 翻译玩家档案
func (tr *Translator) TranslateProfile(p *Profile) *dto.PlayerProfile {
	profile := &dto.PlayerProfile{
		Source:       "enka",
		UID:          p.UID,
		Nickname:     p.PlayerInfo.Nickname,
		Level:        p.PlayerInfo.Level,
		Signature:    p.PlayerInfo.Signature,
		WorldLevel:   p.PlayerInfo.WorldLevel,
		Achievements: p.PlayerInfo.FinishAchievementNum,
		Characters:   make([]dto.CharacterCard, 0, len(p.AvatarInfoList)),
		Abyss:        abyssSummary(p.PlayerInfo),
		Theater:      theaterSummary(p.PlayerInfo),
		Stygian:      stygianSummary(p.PlayerInfo),
	}

	if ch, ok := tr.assets.Character(int64(p.PlayerInfo.ProfilePicture.ID)); ok {
		profile.AvatarURL = enkaUIBase + ch.Icon + ".png"
	}

	missing := map[string]bool{}
	report := func(kind, id string) {
		tr.assets.ReportMissing(kind, id)
		missing[kind] = true
	}

	// 文本 hash → 中文名（未命中返回空串并上报）
	textByHash := func(hash json.Number) (string, bool) {
		num, err := strconv.ParseInt(hash.String(), 10, 64)
		if err != nil {
			return "", false
		}
		return tr.assets.Text(num)
	}

	for _, a := range p.AvatarInfoList {
		card := dto.CharacterCard{
			ID:            int64(a.AvatarID),
			Level:         levelFromProps(a.PropMap),
			Constellation: len(a.TalentIDList),
			Fetter:        a.FetterInfo.ExpLevel,
			Talents:       talentLevels(a.SkillLevelMap),
		}

		if ch, ok := tr.assets.Character(int64(a.AvatarID)); ok {
			card.Name = ch.Name
			card.Element = ch.Element
			card.Rarity = ch.Rarity
			card.IconURL = enkaUIBase + ch.Icon + ".png"
		} else {
			card.Name = fmt.Sprintf("未知角色 #%d", a.AvatarID)
			report(asset.KindCharacter, fmt.Sprint(a.AvatarID))
		}
		// 推荐强化词条（有录入才输出）
		// tierByName：中文词条名 → 属性分级（primary/secondary），供面板属性与圣遗物词条着色
		tierByName := map[string]string{}
		if rec, ok := tr.assets.StatRecommendation(int64(a.AvatarID)); ok && len(rec.SubStats) > 0 {
			card.StatWeights = tr.TranslateStatRecommendation(rec)
			for _, e := range card.StatWeights.SubStats {
				if e.Tier != "" {
					tierByName[e.Name] = e.Tier
				}
			}
		}

		card.Stats = tr.panelStats(a.FightPropMap, card.Element, tierByName)

		// 圣遗物评分入参（V3）：与 card.Relics 同序，装备循环结束后统一算分
		// —— 套装件数（绝缘4）与武器名要等圣遗物/武器收集齐才可判
		scoreInputs := make([]asset.RelicScoreInput, 0, 5)

		for _, eq := range a.EquipList {
			switch eq.Flat.ItemType {
			case "ITEM_WEAPON":
				weaponName, known := textByHash(eq.Flat.NameTextMapHash)
				if !known {
					weaponName = "未知武器"
					report(asset.KindText, eq.Flat.NameTextMapHash.String())
				}
				card.Weapon = &dto.WeaponInfo{
					Name:   weaponName,
					Rarity: eq.Flat.RankLevel,
					Level:  weaponLevel(eq.Weapon),
					Affix:  weaponAffix(eq.Weapon),
				}

			case "ITEM_RELIQUARY":
				if eq.Reliquary == nil {
					continue
				}
				// Enka 内部圣遗物等级范围 [1-21]，对应游戏显示 +0~+20，展示时统一减 1
				relicLevel, _ := eq.Reliquary.Level.Int64()
				relicDisplayLevel := int(relicLevel) - 1
				if relicDisplayLevel < 0 {
					relicDisplayLevel = 0
				}

				// 套装名：从 relic_sets 映射表按 setId 查
				setName := ""
				if rs, ok := tr.assets.RelicSet(eq.Flat.SetID); ok {
					setName = rs.Name
				} else {
					report(asset.KindRelicSet, strconv.Itoa(eq.Flat.SetID))
				}

				// 件名：文本映射表；缺失时用套装名兜底
				itemName, itemKnown := textByHash(eq.Flat.NameTextMapHash)
				if !itemKnown {
					report(asset.KindText, eq.Flat.NameTextMapHash.String())
				}

				// 强化次数解码：appendPropIdList 每个 id = 一次获得/强化
				enhanceCount := map[string]int{}
				for _, affixID := range eq.Reliquary.AppendPropIDList {
					if af, ok := tr.assets.Affix(affixID); ok {
						enhanceCount[af.PropType]++
					} else {
						report(asset.KindProp, strconv.FormatInt(affixID, 10))
					}
				}

				// 部位图标：Enka 的 flat.icon 形如 UI_RelicIcon_15005_4，与角色头像同一图床
				iconURL := ""
				if eq.Flat.Icon != "" {
					iconURL = enkaUIBase + eq.Flat.Icon + ".png"
				}

				relic := dto.RelicInfo{
					Name:    firstNonEmpty(itemName, setName),
					Set:     setName,
					Slot:    tr.assets.RelicSlotName(eq.Flat.EquipType),
					Rarity:  eq.Flat.RankLevel,
					Level:   relicDisplayLevel,
					Pos:     eq.Flat.EquipType,
					IconURL: iconURL,
				}

				// 主词条 + 副词条（附强化次数与属性分级）
				// 评分入参在此处取**原始 float**（StatValue），dto 层的字符串是 %g 格式化结果、有精度损失（ai/08 附2 踩坑 5）
				scoreIn := asset.RelicScoreInput{Slot: eq.Flat.EquipType}
				if eq.Flat.ReliquaryMainstat.MainPropID != "" {
					name, value := tr.statDetail(
						eq.Flat.ReliquaryMainstat.MainPropID,
						eq.Flat.ReliquaryMainstat.StatValue,
					)
					relic.MainStat = fmt.Sprintf("%s +%s", name, value)
					relic.MainStatTier = tierByName[name]
					scoreIn.MainStatName = name
					scoreIn.MainStatValue = eq.Flat.ReliquaryMainstat.StatValue
				}
				for _, sub := range eq.Flat.ReliquarySubstats {
					name, value := tr.statDetail(sub.AppendPropID, sub.StatValue)
					relic.SubStats = append(relic.SubStats, dto.RelicSubStat{
						Name:  name,
						Value: value,
						Count: enhanceCount[sub.AppendPropID],
						Tier:  tierByName[name],
					})
					scoreIn.SubStats = append(scoreIn.SubStats, asset.RelicStatInput{
						Name:  name,
						Value: sub.StatValue,
					})
				}

				card.Relics = append(card.Relics, relic)
				scoreInputs = append(scoreInputs, scoreIn)
			}
		}

		tr.scoreRelics(&card, scoreInputs)

		profile.Characters = append(profile.Characters, card)
	}

	if len(missing) > 0 {
		profile.Warnings = append(profile.Warnings,
			fmt.Sprintf("存在 %d 类未映射的游戏资产，已记录待补录", len(missing)))
	}
	return profile
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// travelerElement 旅行者当前元素（小助手口径的权重按 (角色, 元素) 取）。
// 非旅行者、或拿不到可信元素时返回 "" ⇒ 权重取该角色的通用行（不会算错，只会少一点精度）。
// ⚠️ 档案里的 Element 取自我们 characters 表配置，**不一定是玩家当前的旅行者元素** ✗
//
//	（真实元素在 Enka 的 skillDepotId 里）—— 接上 skillDepotId 是后续小改。
func travelerElement(card *dto.CharacterCard) string {
	switch card.ID {
	case 10000005, 10000007, 10000117, 10000118: // 旅行者（荧/空）与 奇偶·男性/女性
	default:
		return ""
	}
	switch card.Element {
	case "风", "岩", "雷", "草", "水", "火", "冰":
		return card.Element
	}
	return ""
}

// scoreRelics 给角色卡打圣遗物评分（V3，2026-09-27；口径见 teyvat-docs/ai/08）。
// 输入是 translate 层收集的原始 float（非 dto 字符串反解）；该角色未配置权重时
// ScoreRelic 会走默认权重，评分恒可用 —— 但旧库未导入权重表时分数会偏"通用"，可接受。
func (tr *Translator) scoreRelics(card *dto.CharacterCard, inputs []asset.RelicScoreInput) {
	if len(inputs) == 0 || len(card.Relics) != len(inputs) {
		return
	}
	// 套装件数（绝缘4 规则要全量件数）
	setCount := map[string]int{}
	for _, r := range card.Relics {
		if set := firstNonEmpty(r.Set, r.Name); set != "" {
			setCount[set]++
		}
	}
	var total, words, crit float64
	scored := 0
	for i := range inputs {
		in := &inputs[i]
		in.CharacterID = card.ID
		in.Constellation = card.Constellation
		if card.Weapon != nil {
			in.WeaponName = card.Weapon.Name
			in.WeaponAffix = card.Weapon.Affix
		}
		if set := firstNonEmpty(card.Relics[i].Set, card.Relics[i].Name); set != "" {
			in.SetName = set
			in.SetCount = setCount[set]
		}
		// 口径分支（2026-09-27）：默认走小助手口径（RELIC_SCORE_MODE=mp），
		// 设成 miao 即回到原实现（改容器环境变量 + 重启即可回滚，不用发版）。
		if asset.RelicScoreMode() == "mp" {
			if w, ok := tr.assets.MPWeight(card.ID, travelerElement(card)); ok {
				mp, _ := asset.ScoreRelicMP(*in, w)
				r := &card.Relics[i]
				r.Score = mp.Score
				r.ScoreCls = asset.MPRelicClass(mp.Score) // 小助手口径阈值（ACE 40 / MAX 45）
				r.WordCount = mp.WordCount
				r.CritScore = mp.CritScore
				total += mp.Score
				words += mp.WordCount
				crit += mp.CritScore
				scored++
				continue
			}
		}
		res, ok := tr.assets.ScoreRelic(*in)
		if !ok {
			continue
		}
		r := &card.Relics[i]
		r.Score = res.Score
		r.ScorePct = res.Percent
		r.ScoreCls = res.Class
		r.WordCount = res.WordCount
		r.CritScore = res.CritScore
		r.WeightTag = res.WeightTag
		total += res.Score
		words += res.WordCount
		crit += res.CritScore
		scored++
	}
	if scored > 0 {
		cls, avg := asset.RelicSetGrade(total, scored)
		if asset.RelicScoreMode() == "mp" {
			cls, avg = asset.MPSetGrade(total, scored) // 套装档位同样走小助手口径阈值
		}
		card.RelicScoreTotal = total
		card.RelicScoreAvg = avg
		card.RelicScoreClass = cls
		card.RelicWordCount = words
		card.RelicCritScore = crit
	}
}

// statDetail 词条翻译：返回（词条名, 格式化数值，百分比词条自动补 %）
func (tr *Translator) statDetail(propID string, value float64) (string, string) {
	if p, ok := tr.assets.Prop(propID); ok {
		if p.Percent {
			return p.Name, fmt.Sprintf("%g%%", value)
		}
		return p.Name, fmt.Sprintf("%g", value)
	}
	return propID, fmt.Sprintf("%g", value)
}

// panelStatIDs 面板展示的属性行（数字枚举，顺序即展示顺序）
// 仅保留 UI 展示逻辑；词条名、格式（flat/percent）与分级一律查 asset 映射表
var panelStatIDs = []string{"2000", "2001", "2002", "28", "20", "22", "23"}

// panelStats 从 fightPropMap 构造面板属性，全部通过映射表翻译：
// 数字枚举 → fight_props → FIGHT_PROP_XXX → prop_maps（词条名 + percent 格式）
func (tr *Translator) panelStats(props map[string]float64, element string, tierByName map[string]string) []dto.StatInfo {
	get := func(key string) float64 { return props[key] }
	format := func(v float64, percent bool) string {
		if percent {
			return fmt.Sprintf("%.1f%%", v*100)
		}
		return fmt.Sprintf("%.0f", v)
	}

	stats := make([]dto.StatInfo, 0, len(panelStatIDs)+1)
	for _, numID := range panelStatIDs {
		propID, ok := tr.assets.PropIDByNum(numID)
		if !ok {
			continue
		}
		p, ok := tr.assets.Prop(propID)
		if !ok {
			continue
		}
		stats = append(stats, dto.StatInfo{
			Name:  p.Name,
			Value: format(get(numID), p.Percent),
			Tier:  tierByName[p.Name],
		})
	}

	// 元素伤害加成行：元素名来自 characters.element（DB 数据），枚举与名称同样查表
	if propID, ok := tr.assets.PropIDByName(element + "元素伤害加成"); ok {
		if numID, ok := tr.assets.NumIDByPropID(propID); ok {
			if p, ok := tr.assets.Prop(propID); ok {
				stats = append(stats, dto.StatInfo{
					Name:  p.Name,
					Value: format(get(numID), p.Percent),
					Tier:  tierByName[p.Name],
				})
			}
		}
	}
	return stats
}

// levelFromProps 从 propMap 取角色等级（key=4001）
func levelFromProps(props map[string]PropVal) int {
	if v, ok := props[propKeyLevel]; ok {
		var level int
		_, _ = fmt.Sscanf(v.Val, "%d", &level)
		return level
	}
	return 0
}

func weaponLevel(w *WeaponDetail) int {
	if w == nil {
		return 0
	}
	lv, _ := w.Level.Int64()
	return int(lv)
}

// weaponAffix 精炼等级（1~5）
// Enka 的 affixMap value 是 0 基（0 = 精炼 1 阶），因此需 +1
func weaponAffix(w *WeaponDetail) int {
	if w == nil || len(w.AffixMap) == 0 {
		return 0
	}
	maxRank := int64(0)
	for _, rank := range w.AffixMap {
		r, _ := rank.Int64()
		if r > maxRank {
			maxRank = r
		}
	}
	return int(maxRank) + 1
}

// talentLevels 从 skillLevelMap 解析天赋等级
// key 为技能 id，升序依次对应 普通攻击 / 元素战技 / 元素爆发
func talentLevels(m map[string]json.Number) *dto.TalentInfo {
	if len(m) == 0 {
		return nil
	}
	type kv struct {
		id int64
		lv int
	}
	list := make([]kv, 0, len(m))
	for k, v := range m {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		lv, _ := v.Int64()
		list = append(list, kv{id: id, lv: int(lv)})
	}
	if len(list) == 0 {
		return nil
	}
	sort.Slice(list, func(i, j int) bool { return list[i].id < list[j].id })

	info := &dto.TalentInfo{}
	if len(list) > 0 {
		info.Normal = list[0].lv
	}
	if len(list) > 1 {
		info.Skill = list[1].lv
	}
	if len(list) > 2 {
		info.Burst = list[2].lv
	}
	return info
}
