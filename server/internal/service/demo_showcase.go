package service

import (
	"encoding/json"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
)

// DemoShowcaseService 游客演示模式（**不调用 AI 服务**）。
//
// 内容全部来自库：
//   - `demo_profiles`  —— 模拟旅行者账号头（昵称 / 等阶 / 深渊概要 / 提示语）
//   - `demo_characters` —— 4 位演示角色，每位带一张**与真实档案同构**的角色卡 + 预置「AI 分析」
//
// 因此演示页的"思考 + 流式输出"由前端本地回放，服务端只负责发数据 ✓：
// 不调模型、不消耗 token、不查 Enka、不落库会话 ✓。
// 换角色 / 改文案 / 下架（enabled=0）都改库即可，不用发版 ✓。
type DemoShowcaseService struct {
	repo *repository.DemoShowcaseRepository
}

// NewDemoShowcaseService 创建服务；表为空时灌入种子数据 ✓
func NewDemoShowcaseService() *DemoShowcaseService {
	s := &DemoShowcaseService{repo: repository.NewDemoShowcaseRepository()}
	s.ensureSeed()
	return s
}

// Showcase 组装演示页数据（账号头 + 演示角色）
func (s *DemoShowcaseService) Showcase() (*dto.DemoShowcase, error) {
	out := &dto.DemoShowcase{Characters: []dto.DemoCharacterItem{}}

	profile, err := s.repo.Profile()
	if err != nil {
		return nil, err
	}
	if profile != nil {
		out.Notice = profile.Notice
		info := &dto.DemoProfileInfo{
			Nickname:     profile.Nickname,
			Signature:    profile.Signature,
			Level:        profile.Level,
			WorldLevel:   profile.WorldLevel,
			Achievements: profile.Achievements,
		}
		// 深渊概要：库里留空 ⇒ 前端整块不渲染 ✓（"没有" ≠ "0 层"）
		if profile.AbyssJSON != "" {
			var abyss dto.AbyssSummary
			if err := json.Unmarshal([]byte(profile.AbyssJSON), &abyss); err == nil {
				info.Abyss = &abyss
			} else {
				logger.L().Warnw("demo profile: abyss json invalid", "err", err)
			}
		}
		out.Profile = info
	}

	rows, err := s.repo.ListEnabledCharacters()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		card := json.RawMessage(row.CardJSON)
		if !json.Valid(card) {
			// 库里配错 JSON 时**跳过这一条**，别让整页挂掉 ✓
			logger.L().Warnw("demo character: card json invalid", "char_key", row.CharKey)
			continue
		}
		out.Characters = append(out.Characters, dto.DemoCharacterItem{
			CharKey:  row.CharKey,
			Sort:     row.Sort,
			Name:     row.Name,
			Element:  row.Element,
			Rarity:   row.Rarity,
			Role:     row.Role,
			Card:     card,
			Analysis: row.Analysis,
		})
	}
	return out, nil
}

// ensureSeed 两张表分别为空时灌默认演示数据。
//
// 口径：**只在表为空时灌**（天然幂等 ✓）——运营删改过内容之后重启服务不会被覆盖 ✓。
// 演示档案的数字与"分析"结论刻意与资产库的攻略卡口径对齐（武器名 / 套装名 / 主词条 / 词条优先级），
// 这样演示内容看起来是真的体检，而不是随手编的 ✗。
func (s *DemoShowcaseService) ensureSeed() {
	s.seedProfile()
	s.seedCharacters()
}

func (s *DemoShowcaseService) seedProfile() {
	count, err := s.repo.CountProfiles()
	if err != nil {
		logger.L().Warnw("demo profile: count failed", "err", err)
		return
	}
	if count > 0 {
		return
	}
	items := []model.DemoProfile{{
		ID:           1,
		Nickname:     "旅行者（游客）",
		Signature:    "演示账号 · 数据为演示内容",
		Level:        57,
		WorldLevel:   8,
		Achievements: 812,
		AbyssJSON:    `{"floor":12,"room":2,"stars":33}`,
		Notice:       "以下为演示内容：角色档案与「分析」都是预置示例，不是实时账号数据。登录并绑定 UID 后，才能看到你自己角色的真实档案。",
		Enabled:      true,
	}}
	if err := s.repo.SeedProfiles(items); err != nil {
		logger.L().Errorw("demo profile: seed failed", "err", err)
		return
	}
	logger.L().Infow("demo profile seeded", "count", len(items))
}

func (s *DemoShowcaseService) seedCharacters() {
	count, err := s.repo.CountCharacters()
	if err != nil {
		logger.L().Warnw("demo character: count failed", "err", err)
		return
	}
	if count > 0 {
		return
	}
	if err := s.repo.SeedCharacters(demoCharacters()); err != nil {
		logger.L().Errorw("demo character: seed failed", "err", err)
		return
	}
	logger.L().Infow("demo characters seeded", "count", 4)
}

// demoCharacters 4 位演示角色（覆盖不同定位与元素，且**都在资产库里** ✓：
// 攻略卡 + 推荐词条权重齐全，所以演示档案与"分析"能和真实口径对得上 ✓）
//
//	玛薇卡（火 5★ 站场主C）· 茜特菈莉（冰 5★ 盾辅）· 薇斯纳（风 5★ 星扩散输出）· 阿罗夏（雷 4★ 加攻治疗）
func demoCharacters() []model.DemoCharacter {
	return []model.DemoCharacter{
		{
			CharKey: "mavuika", Sort: 1,
			Name: "玛薇卡", Element: "火", Rarity: 5, Role: "站场主C（可切后台）",
			CardJSON: demoCard(dto.CharacterCard{
				ID: 10000106, Name: "玛薇卡", Element: "火", Rarity: 5, Level: 90, Constellation: 1, Fetter: 10,
				Talents: &dto.TalentInfo{Normal: 1, Skill: 9, Burst: 10},
				Stats: []dto.StatInfo{
					st("生命值", "18,240", ""),
					st("攻击力", "2,264", "primary"),
					st("防御力", "784", ""),
					st("元素精通", "118", "primary"),
					st("暴击率", "68.2%", "primary"),
					st("暴击伤害", "182.4%", "primary"),
					st("元素充能效率", "112.0%", ""),
					st("火元素伤害加成", "46.6%", "primary"),
				},
				StatWeights: &dto.StatRecommendation{
					SubStats: []dto.StatWeightEntry{
						{Name: "暴击率", Weight: 100, Tier: "primary"},
						{Name: "暴击伤害", Weight: 100, Tier: "primary"},
						{Name: "攻击力百分比", Weight: 75, Tier: "primary"},
						{Name: "元素精通", Weight: 75, Tier: "primary"},
						{Name: "攻击力", Weight: 50, Tier: "secondary"},
						{Name: "火元素伤害加成", Weight: 5, Tier: "primary"},
					},
					MainStats: map[string]string{"sand": "攻击力", "goblet": "火元素伤害加成", "circlet": "暴击率/暴击伤害"},
				},
				Weapon: &dto.WeaponInfo{Name: "焚曜千阳", Rarity: 5, Level: 90, Affix: 1},
				Relics: []dto.RelicInfo{
					rl("黑曜秘典", "生之花", "EQUIP_BRACER", "生命值", "secondary", 52.4, "S", 8.6, 21.8),
					rl("黑曜秘典", "死之羽", "EQUIP_NECKLACE", "攻击力", "secondary", 55.2, "S", 9.2, 24.6),
					rl("黑曜秘典", "时之沙", "EQUIP_SHOES", "攻击力", "primary", 51.8, "S", 8.4, 20.4),
					rl("黑曜秘典", "空之杯", "EQUIP_RING", "火元素伤害加成", "primary", 54.9, "S", 9.6, 22.6),
					rl("黑曜秘典", "理之冠", "EQUIP_DRESS", "暴击伤害", "primary", 53.7, "S", 10.7, 22.8),
				},
				RelicScoreTotal: 268.0, RelicScoreAvg: 53.6, RelicScoreClass: "S",
				RelicWordCount: 46.5, RelicCritScore: 112.2,
			}),
			Analysis: `结论：成型档。1 命的玛薇卡已经能稳定打深渊 12 层，圣遗物平均 53.6 分（S 档），双暴 68.2% / 182.4% 达标。

接下来按优先级做两件事：
1. 元素精通只有 118。蒸发体系的毕业线在 350 左右（含队伍 buff），你现在打蒸发时反应伤害偏低——短期靠队友的精通 buff 或「教官」补，长期考虑把时之沙换成元素精通主词条（副词条仍然要双暴）。
2. 天赋维持「普攻 1 级」是对的，资源全压元素爆发：爆发直接影响坠日斩，优先拉满 10 级；战技 9 级可以停。

另外提醒：玛薇卡的爆发靠战意、不靠充能，元素充能效率对她是无效词条——副词条上看到充能就当歪了，别为它牺牲双暴。`,
			Enabled: true,
		},
		{
			CharKey: "citlali", Sort: 2,
			Name: "茜特菈莉", Element: "冰", Rarity: 5, Role: "盾辅（挂冰 / 水火减抗）",
			CardJSON: demoCard(dto.CharacterCard{
				ID: 10000107, Name: "茜特菈莉", Element: "冰", Rarity: 5, Level: 90, Constellation: 0, Fetter: 8,
				Talents: &dto.TalentInfo{Normal: 1, Skill: 10, Burst: 1},
				Stats: []dto.StatInfo{
					st("生命值", "21,430", ""),
					st("攻击力", "1,246", ""),
					st("防御力", "1,042", ""),
					st("元素精通", "612", "primary"),
					st("暴击率", "41.6%", "secondary"),
					st("暴击伤害", "62.4%", "secondary"),
					st("元素充能效率", "186.3%", "primary"),
				},
				StatWeights: &dto.StatRecommendation{
					SubStats: []dto.StatWeightEntry{
						{Name: "元素精通", Weight: 100, Tier: "primary"},
						{Name: "元素充能效率", Weight: 70, Tier: "secondary"},
						{Name: "暴击率", Weight: 50, Tier: "secondary"},
						{Name: "暴击伤害", Weight: 50, Tier: "secondary"},
						{Name: "冰元素伤害加成", Weight: 5, Tier: "primary"},
					},
					MainStats: map[string]string{"sand": "元素充能效率", "goblet": "元素精通", "circlet": "暴击率"},
				},
				Weapon: &dto.WeaponInfo{Name: "祭星者之望", Rarity: 5, Level: 90, Affix: 1},
				Relics: []dto.RelicInfo{
					rl("烬城勇者绘卷", "生之花", "EQUIP_BRACER", "生命值", "secondary", 41.2, "B", 7.4, 6.8),
					rl("烬城勇者绘卷", "死之羽", "EQUIP_NECKLACE", "攻击力", "secondary", 43.6, "B", 7.9, 8.2),
					rl("烬城勇者绘卷", "时之沙", "EQUIP_SHOES", "元素充能效率", "primary", 42.4, "B", 7.6, 7.4),
					rl("烬城勇者绘卷", "空之杯", "EQUIP_RING", "元素精通", "primary", 44.8, "B", 8.8, 9.6),
					rl("烬城勇者绘卷", "理之冠", "EQUIP_DRESS", "暴击率", "primary", 42.0, "B", 9.5, 6.2),
				},
				RelicScoreTotal: 214.0, RelicScoreAvg: 42.8, RelicScoreClass: "B",
				RelicWordCount: 41.2, RelicCritScore: 38.2,
			}),
			Analysis: `结论：辅助不该拿双暴评分来看。这套平均 42.8 分（B 档）在盾辅里是正常的——护盾量只吃元素精通，你现在 612 精通撑得住盾，暴击双暴低不影响她的价值。

两件事：
1. 元素充能效率 186%，刚好压在「单冰 + 队内一把西风武器」的毕业线（180%）上。如果队里一把西风都没有，需求要再加 15% 左右，那种配队下建议换充能沙；反过来，如果你的队伍不需要她挂冰（直伤队），充能可以完全不管，词条全换精通。
2. 天赋只升元素战技是对的——爆发和普攻不用升，命座的增益也不吃爆发等级。

武器带「祭星者之望」时精通越高收益越好；若换成「尘光七谕」，记得把攻击力堆到 2600 附近，那是它增伤的上限。`,
			Enabled: true,
		},
		{
			CharKey: "vysna", Sort: 3,
			Name: "薇斯纳", Element: "风", Rarity: 5, Role: "星扩散站场输出",
			CardJSON: demoCard(dto.CharacterCard{
				ID: 10000143, Name: "薇斯纳", Element: "风", Rarity: 5, Level: 90, Constellation: 0, Fetter: 6,
				Talents: &dto.TalentInfo{Normal: 6, Skill: 10, Burst: 8},
				Stats: []dto.StatInfo{
					st("生命值", "16,880", ""),
					st("攻击力", "2,680", "primary"),
					st("防御力", "826", ""),
					st("元素精通", "96", "secondary"),
					st("暴击率", "71.5%", "primary"),
					st("暴击伤害", "178.9%", "primary"),
					st("元素充能效率", "118.0%", ""),
				},
				StatWeights: &dto.StatRecommendation{
					SubStats: []dto.StatWeightEntry{
						{Name: "暴击率", Weight: 100, Tier: "primary"},
						{Name: "暴击伤害", Weight: 100, Tier: "primary"},
						{Name: "攻击力百分比", Weight: 75, Tier: "primary"},
						{Name: "攻击力", Weight: 50, Tier: "secondary"},
						{Name: "元素精通", Weight: 50, Tier: "secondary"},
					},
					MainStats: map[string]string{"sand": "攻击力", "goblet": "攻击力", "circlet": "暴击伤害/暴击率"},
				},
				Weapon: &dto.WeaponInfo{Name: "蝶变", Rarity: 5, Level: 90, Affix: 1},
				Relics: []dto.RelicInfo{
					rl("血红之证", "生之花", "EQUIP_BRACER", "生命值", "secondary", 58.4, "SS", 9.2, 24.2),
					rl("血红之证", "死之羽", "EQUIP_NECKLACE", "攻击力", "secondary", 60.2, "SS", 9.8, 26.4),
					rl("血红之证", "时之沙", "EQUIP_SHOES", "攻击力", "primary", 57.6, "SS", 9.1, 23.8),
					rl("血红之证", "空之杯", "EQUIP_RING", "攻击力", "primary", 61.8, "SS", 10.4, 25.6),
					rl("血红之证", "理之冠", "EQUIP_DRESS", "暴击伤害", "primary", 58.0, "SS", 10.3, 21.6),
				},
				RelicScoreTotal: 296.0, RelicScoreAvg: 59.2, RelicScoreClass: "SS",
				RelicWordCount: 48.8, RelicCritScore: 121.6,
			}),
			Analysis: `结论：毕业档。攻击力 2,680、双暴 71.5% / 178.9%，圣遗物平均 59.2 分（SS 档），星扩散队里她基本没有短板。

下一步：
1. 天赋优先级是 元素战技 ＞ 元素爆发 ＞ 普通攻击：普攻停在 6 级，资源全给战技——每级都直接加成翔风剑；爆发 8 级可以再往 10 推。
2. 单风队里靠「蝶变」的回能就能每两轮一次爆发，不用额外堆充能；想每轮一次爆发得靠双风共鸣 + 更高充能，属于进阶玩法。
3. 低命时反应星扩散占比不低：队伍如果缺星烁反应增伤 buff，可以把沙或杯的其中一件换成元素精通主词条，总伤反而更高。

配队上优先找能减冰抗 / 风抗的队友（沃雅妮莎这类），她的输出对减抗吃得很深。`,
			Enabled: true,
		},
		{
			CharKey: "arocha", Sort: 4,
			Name: "阿罗夏", Element: "雷", Rarity: 4, Role: "加攻 / 治疗辅助",
			CardJSON: demoCard(dto.CharacterCard{
				ID: 10000148, Name: "阿罗夏", Element: "雷", Rarity: 4, Level: 80, Constellation: 2, Fetter: 4,
				Talents: &dto.TalentInfo{Normal: 1, Skill: 8, Burst: 1},
				Stats: []dto.StatInfo{
					st("生命值", "19,600", ""),
					st("攻击力", "1,240", ""),
					st("防御力", "918", ""),
					st("元素精通", "88", ""),
					st("暴击率", "52.4%", "secondary"),
					st("暴击伤害", "96.8%", "secondary"),
					st("元素充能效率", "214.0%", "primary"),
				},
				StatWeights: &dto.StatRecommendation{
					SubStats: []dto.StatWeightEntry{
						{Name: "攻击力百分比", Weight: 100, Tier: "primary"},
						{Name: "元素充能效率", Weight: 75, Tier: "primary"},
						{Name: "暴击率", Weight: 50, Tier: "secondary"},
						{Name: "暴击伤害", Weight: 50, Tier: "secondary"},
						{Name: "攻击力", Weight: 50, Tier: "secondary"},
					},
					MainStats: map[string]string{"sand": "攻击力", "goblet": "攻击力", "circlet": "暴击率/暴击伤害"},
				},
				Weapon: &dto.WeaponInfo{Name: "寒息", Rarity: 4, Level: 90, Affix: 5},
				Relics: []dto.RelicInfo{
					rl("炉火融炼之心", "生之花", "EQUIP_BRACER", "生命值", "secondary", 48.2, "A", 8.4, 15.2),
					rl("炉火融炼之心", "死之羽", "EQUIP_NECKLACE", "攻击力", "secondary", 50.6, "A", 8.8, 17.8),
					rl("炉火融炼之心", "时之沙", "EQUIP_SHOES", "攻击力", "primary", 47.8, "A", 8.2, 14.6),
					rl("炉火融炼之心", "空之杯", "EQUIP_RING", "攻击力", "primary", 51.2, "A", 9.0, 18.4),
					rl("炉火融炼之心", "理之冠", "EQUIP_DRESS", "暴击率", "primary", 50.2, "A", 9.2, 22.4),
				},
				RelicScoreTotal: 248.0, RelicScoreAvg: 49.6, RelicScoreClass: "A",
				RelicWordCount: 43.6, RelicCritScore: 88.4,
			}),
			Analysis: `结论：定位是「加攻 + 治疗」的辅助，4★ 里性价比很高，但练度还差两步——等级停在 80，天赋只有战技 8 级。

1. 充能没问题：214% 已经过了「70 点能量的每轮爆发线」（解锁 1 命后需求降到 55 点）。接下来把元素战技补到 10 级，直接抬高给队友的攻击力数值。
2. 圣遗物平均 49.6 分（A 档），套装用「炉火融炼之心」靠星超导给全队增伤是对的；主词条按攻略卡是攻击 / 攻击 / 暴击，副词条继续优先充能。
3. 天赋只升战技——普攻和爆发都不用升，治疗量和加攻都只吃战技等级。

一句话：等级拉到 90、战技补到 10，她就能稳定进桑多涅的双冰双雷星超导队。`,
			Enabled: true,
		},
	}
}

// ---------- 种子构造小工具（让上面的角色卡保持可读） ----------

func demoCard(card dto.CharacterCard) string {
	b, err := json.Marshal(card)
	if err != nil {
		logger.L().Errorw("demo character: marshal card failed", "name", card.Name, "err", err)
		return "{}"
	}
	return string(b)
}

func st(name, value, tier string) dto.StatInfo {
	return dto.StatInfo{Name: name, Value: value, Tier: tier}
}

func rl(set, slot, pos, main, mainTier string, score float64, cls string, words, crit float64) dto.RelicInfo {
	return dto.RelicInfo{
		Name:         set + "·" + slot,
		Set:          set,
		Slot:         slot,
		Rarity:       5,
		Level:        20,
		Pos:          pos,
		MainStat:     main,
		MainStatTier: mainTier,
		Score:        score,
		ScoreCls:     cls,
		WordCount:    words,
		CritScore:    crit,
	}
}
