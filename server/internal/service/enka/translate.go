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
		// 推荐强化词条（有录入才输出，statKey 翻译为中文词条名并按权重降序）
		// tierByName：中文词条名 → 属性分级（primary/secondary），供面板属性与圣遗物词条着色
		tierByName := map[string]string{}
		if rec, ok := tr.assets.StatRecommendation(int64(a.AvatarID)); ok && len(rec.SubStats) > 0 {
			entries := make([]dto.StatWeightEntry, 0, len(rec.SubStats))
			for _, it := range rec.SubStats {
				name := it.StatKey
				if p, ok2 := tr.assets.Prop(it.StatKey); ok2 {
					name = p.Name
				}
				entries = append(entries, dto.StatWeightEntry{Name: name, Weight: it.Weight, Tier: it.Tier})
				if it.Tier != "" {
					tierByName[name] = it.Tier
				}
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Weight > entries[j].Weight })
			card.StatWeights = &dto.StatRecommendation{SubStats: entries, MainStats: rec.MainStats}
		}

		card.Stats = tr.panelStats(a.FightPropMap, card.Element, tierByName)

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

				relic := dto.RelicInfo{
					Name:   firstNonEmpty(itemName, setName),
					Set:    setName,
					Slot:   tr.assets.RelicSlotName(eq.Flat.EquipType),
					Rarity: eq.Flat.RankLevel,
					Level:  relicDisplayLevel,
					Pos:    eq.Flat.EquipType,
				}

				// 主词条 + 副词条（附强化次数与属性分级）
				if eq.Flat.ReliquaryMainstat.MainPropID != "" {
					name, value := tr.statDetail(
						eq.Flat.ReliquaryMainstat.MainPropID,
						eq.Flat.ReliquaryMainstat.StatValue,
					)
					relic.MainStat = fmt.Sprintf("%s +%s", name, value)
					relic.MainStatTier = tierByName[name]
				}
				for _, sub := range eq.Flat.ReliquarySubstats {
					name, value := tr.statDetail(sub.AppendPropID, sub.StatValue)
					relic.SubStats = append(relic.SubStats, dto.RelicSubStat{
						Name:  name,
						Value: value,
						Count: enhanceCount[sub.AppendPropID],
						Tier:  tierByName[name],
					})
				}

				card.Relics = append(card.Relics, relic)
			}
		}

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
