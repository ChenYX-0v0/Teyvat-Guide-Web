package enka

// 翻译层：Enka 原始数据 → 人类可读的 dto
// 名称依赖 AssetService（MySQL 映射），未命中走 fallback 并收集缺漏

import (
	"encoding/json"
	"fmt"
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
		card.Stats = panelStats(a.FightPropMap, card.Element)

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
					Rarity: eq.Flat.RankLevel,
					Level:  relicDisplayLevel,
					Pos:    eq.Flat.EquipType,
				}

				// 主词条 + 副词条（附强化次数）
				if eq.Flat.ReliquaryMainstat.MainPropID != "" {
					name, value := tr.statDetail(
						eq.Flat.ReliquaryMainstat.MainPropID,
						eq.Flat.ReliquaryMainstat.StatValue,
					)
					relic.MainStat = fmt.Sprintf("%s +%s", name, value)
				}
				for _, sub := range eq.Flat.ReliquarySubstats {
					name, value := tr.statDetail(sub.AppendPropID, sub.StatValue)
					relic.SubStats = append(relic.SubStats, dto.RelicSubStat{
						Name:  name,
						Value: value,
						Count: enhanceCount[sub.AppendPropID],
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

// panelStats 从 fightPropMap 构造面板属性
// FIGHT_PROP 数字枚举：2000=生命 2001=攻击 2002=防御 28=精通 20=暴击率 22=暴击伤害 23=充能
// 元素伤害加成：41=风 42=岩 43=雷 44=草 45=水 46=火 47=冰
func panelStats(props map[string]float64, element string) []dto.StatInfo {
	get := func(key string) float64 { return props[key] }
	percent := func(key string) string { return fmt.Sprintf("%.1f%%", get(key)*100) }
	flat := func(key string) string { return fmt.Sprintf("%.0f", get(key)) }

	stats := []dto.StatInfo{
		{Name: "生命值", Value: flat("2000")},
		{Name: "攻击力", Value: flat("2001")},
		{Name: "防御力", Value: flat("2002")},
		{Name: "元素精通", Value: flat("28")},
		{Name: "暴击率", Value: percent("20")},
		{Name: "暴击伤害", Value: percent("22")},
		{Name: "元素充能效率", Value: percent("23")},
	}

	// 按角色元素取对应的元素伤害加成
	dmgProp := map[string]string{
		"火": "46", "水": "45", "风": "41", "雷": "43", "草": "44", "岩": "42", "冰": "47",
	}
	if key, ok := dmgProp[element]; ok {
		stats = append(stats, dto.StatInfo{
			Name:  element + "元素伤害加成",
			Value: percent(key),
		})
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

// weaponAffix 取最高精炼等级
func weaponAffix(w *WeaponDetail) int {
	if w == nil {
		return 0
	}
	maxRank := 0
	for _, rank := range w.AffixMap {
		r, _ := rank.Int64()
		if int(r) > maxRank {
			maxRank = int(r)
		}
	}
	return maxRank
}
