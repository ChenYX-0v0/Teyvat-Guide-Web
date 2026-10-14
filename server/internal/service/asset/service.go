package asset

// AssetService 映射数据服务
// - 启动时从 MySQL 全量加载进内存，翻译零 SQL
// - 未命中异步写入 missings 供补录（表名 2026-09-27 校正：无 `asset_` 前缀 ✗）
// - Import 后自动 reload，立即生效无需重启

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"teyvat-guide/server/internal/pkg/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 缺漏收集的类别
const (
	KindCharacter = "character"
	KindWeapon    = "weapon"
	KindRelicSet  = "relic_set"
	KindText      = "text"
	KindProp      = "prop"
)

var ErrDBNotInitialized = errors.New("数据库未初始化，无法管理映射数据")

type Service struct {
	db *gorm.DB

	mu         sync.RWMutex
	characters map[int64]Character
	// aliases 角色别名 → 角色名（「雷神」→「雷电将军」✓，2026-10-14 加 ✓）
	//   真源 scripts/data/character_aliases.json ✓（表：character_aliases ✓，`-aliases-file` 导入 ✓）
	aliases      map[string]string
	weapons      map[int64]Weapon
	relicSets    map[int]RelicSet
	texts        map[int64]string
	props        map[string]PropMap
	propIDByName map[string]string // 词条名 → PropID
	fightProps   map[string]string // 数字枚举 → PropID（正向）
	numByFight   map[string]string // PropID → 数字枚举（反向）
	relicSlots   map[string]string // 部位枚举 → 中文部位名
	affixes      map[int64]Affix
	statRecs     map[int64]StatRecommendation
	// 圣遗物评分权重（V3，2026-09-27）：与 statRecs 语义不同，见 model.go 注释
	relicWeights map[int64]CharacterRelicWeight
	relicRules   map[int64][]CharacterRelicWeightRule
	// 小助手口径权重（7 维，主键 = (角色, 元素)）——与 miao 那套并存，口径见 relic_score_mp.go
	mpWeights  map[mpWeightKey]CharacterRelicWeightMP
	setEffects map[int][]RelicSetEffect // 套装 ID → 2/4 件效果（按件数升序）
}

// New 创建服务；db 为 nil 时降级为空映射（翻译全部走 fallback）
func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Load 全量加载映射到内存
func (s *Service) Load() error {
	if s.db == nil {
		return nil
	}

	var chars []Character
	if err := s.db.Find(&chars).Error; err != nil {
		return err
	}
	var weapons []Weapon
	if err := s.db.Find(&weapons).Error; err != nil {
		return err
	}
	var sets []RelicSet
	if err := s.db.Find(&sets).Error; err != nil {
		return err
	}
	var setEffects []RelicSetEffect
	if err := s.db.Find(&setEffects).Error; err != nil {
		return err
	}
	var texts []TextMapEntry
	if err := s.db.Find(&texts).Error; err != nil {
		return err
	}
	var props []PropMap
	if err := s.db.Find(&props).Error; err != nil {
		return err
	}
	var affixes []Affix
	if err := s.db.Find(&affixes).Error; err != nil {
		return err
	}
	var fightProps []FightProp
	if err := s.db.Find(&fightProps).Error; err != nil {
		return err
	}
	var relicSlots []RelicSlot
	if err := s.db.Find(&relicSlots).Error; err != nil {
		return err
	}
	var statRecRows []CharacterStatWeights
	if err := s.db.Find(&statRecRows).Error; err != nil {
		return err
	}
	var relicWeightRows []CharacterRelicWeight
	if err := s.db.Find(&relicWeightRows).Error; err != nil {
		return err
	}
	var relicRuleRows []CharacterRelicWeightRule
	if err := s.db.Find(&relicRuleRows).Error; err != nil {
		return err
	}
	var mpWeightRows []CharacterRelicWeightMP
	if err := s.db.Find(&mpWeightRows).Error; err != nil {
		return err
	}

	charMap := make(map[int64]Character, len(chars))
	for _, c := range chars {
		charMap[c.ID] = c
	}
	weaponMap := make(map[int64]Weapon, len(weapons))
	for _, w := range weapons {
		weaponMap[w.ID] = w
	}
	setMap := make(map[int]RelicSet, len(sets))
	for _, r := range sets {
		setMap[r.SetID] = r
	}
	textMap := make(map[int64]string, len(texts))
	for _, tx := range texts {
		textMap[tx.Hash] = tx.Text
	}
	propMap := make(map[string]PropMap, len(props))
	fightByNum := make(map[string]string, len(fightProps))
	numByFight := make(map[string]string, len(fightProps))
	for _, f := range fightProps {
		fightByNum[f.NumID] = f.PropID
		numByFight[f.PropID] = f.NumID
	}
	propIDByName := make(map[string]string, len(props))
	for _, p := range props {
		propMap[p.PropID] = p
		// 词条名 → PropID：**同名冲突时优先保留"有数字枚举"的那个**（防御性保留 ✓）。
		//
		// 反例（2026-09-26 踩到）：`FIGHT_PROP_FIRE_ADD` 与 `FIGHT_PROP_FIRE_ADD_HURT`
		// 都叫「火元素伤害加成」，而面板属性行要用数字枚举回查数值 —— 若选中没有枚举的那个，
		// 整行会被跳过，表现为「角色面板里 元素伤害加成 整项消失」✗。
		// 顺序敏感：所以先建 numByFight 再建本表 ✓。
		//
		// 2026-09-27 已清理数据：库与种子里只留**真 key（`_*_ADD_HURT`）** ✓（删掉 8 条旧 `_ADD` 行，
		// 见 deploy/out/clean-dup-add-props.js）；下面的裁决逻辑仍保留，作为"数据再出错"的兜底 ✓。
		// ⚠️ `FIGHT_PROP_HEAL_ADD` / `FIGHT_PROP_HEALED_ADD` **没有** HURT 兄弟，是正当 key ✓。
		old, exists := propIDByName[p.Name]
		if !exists {
			propIDByName[p.Name] = p.PropID
			continue
		}
		if _, oldHasNum := numByFight[old]; oldHasNum {
			continue // 已选中的那个有枚举，保持不动（先到先得）
		}
		if _, hasNum := numByFight[p.PropID]; hasNum {
			propIDByName[p.Name] = p.PropID // 当前这个才有枚举，换成它
		}
	}
	slotMap := make(map[string]string, len(relicSlots))
	for _, sl := range relicSlots {
		slotMap[sl.Pos] = sl.Name
	}
	affixMap := make(map[int64]Affix, len(affixes))
	for _, a := range affixes {
		affixMap[a.ID] = a
	}
	recMap := make(map[int64]StatRecommendation, len(statRecRows))
	for _, row := range statRecRows {
		var rec StatRecommendation
		if err := json.Unmarshal([]byte(row.Payload), &rec); err == nil {
			recMap[row.CharacterID] = rec
		}
	}
	setEffectMap := make(map[int][]RelicSetEffect, len(setEffects))
	for _, e := range setEffects {
		setEffectMap[e.SetID] = append(setEffectMap[e.SetID], e)
	}
	relicWeightMap := make(map[int64]CharacterRelicWeight, len(relicWeightRows))
	for _, rw := range relicWeightRows {
		relicWeightMap[rw.CharacterID] = rw
	}
	relicRuleMap := make(map[int64][]CharacterRelicWeightRule, len(relicRuleRows))
	for _, rr := range relicRuleRows {
		relicRuleMap[rr.CharacterID] = append(relicRuleMap[rr.CharacterID], rr)
	}
	// 小助手口径权重：主键 (角色, 元素)；非旅行者 element 为空串
	//
	// ⚠️ 2026-09-27 起**真源已改为推荐属性 payload 的 score 块** ✓（见下方覆盖逻辑）
	// —— 这张表降级为**过渡兜底**（表不删，但不是编辑入口）
	mpWeightMap := make(map[mpWeightKey]CharacterRelicWeightMP, len(mpWeightRows))
	for _, mw := range mpWeightRows {
		mpWeightMap[mpWeightKey{ID: mw.CharacterID, Element: mw.Element}] = mw
	}
	// 用推荐属性里的权重**覆盖**上表（有配置的角色以推荐属性为准 ✓）
	// 2026-09-27 起：评分权重就是 `subStats[].weight`（一个词条一个数字 ✓）；老结构走过渡兼容 ✓
	for _, row := range statRecRows {
		rec, ok := recMap[row.CharacterID]
		if !ok {
			continue
		}
		if w, ok := mpWeightFromRec(row.CharacterID, rec, ""); ok {
			mpWeightMap[mpWeightKey{ID: row.CharacterID}] = w
		}
		for elem := range rec.ByElement {
			if w, ok := mpWeightFromRec(row.CharacterID, rec, elem); ok {
				mpWeightMap[mpWeightKey{ID: row.CharacterID, Element: elem}] = w
			}
		}
		if rec.Score != nil { // 过渡兼容：老 payload 的 score 块（含 byElement）
			for elem, dims := range rec.Score.ByElement {
				mpWeightMap[mpWeightKey{ID: row.CharacterID, Element: elem}] = mpWeightFromDims(row.CharacterID, dims)
			}
			if len(rec.Score.Dims) > 0 {
				mpWeightMap[mpWeightKey{ID: row.CharacterID}] = mpWeightFromDims(row.CharacterID, rec.Score.Dims)
			}
		}
	}
	// 规则顺序要稳定（map/DB 返回顺序不保证；同一角色多条规则可叠加，展示标签也要可复现）
	for id, list := range relicRuleMap {
		sort.Slice(list, func(i, j int) bool {
			if list[i].CondType != list[j].CondType {
				return list[i].CondType < list[j].CondType
			}
			return list[i].StatKey < list[j].StatKey
		})
		relicRuleMap[id] = list
	}
	// 按件数升序：渲染出的资料文本要可复现（map 遍历无序，DB 返回顺序也不保证）
	for id, list := range setEffectMap {
		sort.Slice(list, func(i, j int) bool { return list[i].Piece < list[j].Piece })
		setEffectMap[id] = list
	}

	// 角色别名（「雷神」→「雷电将军」✓）—— ⚠️ 表不存在时**不报错** ✓（迁移前的老库也要能启动 ✓）
	aliasMap := map[string]string{}
	var aliasRows []CharacterAlias
	if err := s.db.Find(&aliasRows).Error; err != nil {
		logger.L().Warnw("character_aliases 读取失败（忽略 ✓）", "err", err)
	}
	for _, a := range aliasRows {
		if a.Alias != "" && a.Name != "" {
			aliasMap[a.Alias] = a.Name
		}
	}

	s.mu.Lock()
	s.characters = charMap
	s.aliases = aliasMap
	s.weapons = weaponMap
	s.relicSets = setMap
	s.texts = textMap
	s.mpWeights = mpWeightMap
	s.props = propMap
	s.propIDByName = propIDByName
	s.fightProps = fightByNum
	s.numByFight = numByFight
	s.relicSlots = slotMap
	s.affixes = affixMap
	s.statRecs = recMap
	s.relicWeights = relicWeightMap
	s.relicRules = relicRuleMap
	s.setEffects = setEffectMap
	s.mu.Unlock()

	logger.L().Infow("asset maps loaded",
		"characters", len(chars), "weapons", len(weapons),
		"relicSets", len(sets), "relicSetEffects", len(setEffects),
		"texts", len(texts), "props", len(props),
		"fightProps", len(fightProps), "affixes", len(affixes), "statRecs", len(recMap),
		"aliases", len(aliasMap),
		"relicWeights", len(relicWeightMap), "relicRules", len(relicRuleRows))
	return nil
}

// Reload 重新加载（import / 管理接口触发）
func (s *Service) Reload() error { return s.Load() }

// ---------- 查询接口（内存） ----------

func (s *Service) Character(id int64) (Character, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.characters[id]
	return c, ok
}

func (s *Service) Weapon(id int64) (Weapon, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.weapons[id]
	return w, ok
}

func (s *Service) RelicSet(id int) (RelicSet, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.relicSets[id]
	return r, ok
}

// RelicSetEffects 按套装 ID 取 2/4 件效果（按件数升序）。没有资料时返回 nil。
// 返回副本，调用方持有期间不会与 reload 竞争。
func (s *Service) RelicSetEffects(setID int) []RelicSetEffect {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.setEffects[setID]
	if len(list) == 0 {
		return nil
	}
	out := make([]RelicSetEffect, len(list))
	copy(out, list)
	return out
}

func (s *Service) Text(hash int64) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	text, ok := s.texts[hash]
	return text, ok
}

func (s *Service) Prop(propID string) (PropMap, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.props[propID]
	return p, ok
}

// PropIDByNum 数字枚举 → PropID（如 "2000" → FIGHT_PROP_HP）
func (s *Service) PropIDByNum(numID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.fightProps[numID]
	return id, ok
}

// NumIDByPropID PropID → 数字枚举（如 FIGHT_PROP_WATER_ADD_HURT → "42"）
func (s *Service) NumIDByPropID(propID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.numByFight[propID]
	return id, ok
}

// RelicSlotName 部位枚举 → 中文部位名（未映射返回原枚举）
func (s *Service) RelicSlotName(pos string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if name, ok := s.relicSlots[pos]; ok {
		return name
	}
	return pos
}

// PropIDByName 词条名 → PropID（元素伤害加成行按元素名动态查表用）
func (s *Service) PropIDByName(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.propIDByName[name]
	return id, ok
}

// Affix 按词缀 id 查词条类型
func (s *Service) Affix(id int64) (Affix, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.affixes[id]
	return a, ok
}

// StatRecommendation 按角色查推荐强化词条
func (s *Service) StatRecommendation(characterID int64) (StatRecommendation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.statRecs[characterID]
	return rec, ok
}

// RelicWeight 按角色查圣遗物评分权重（V3）。未配置时返回 false，算分侧会走默认权重。
func (s *Service) RelicWeight(characterID int64) (CharacterRelicWeight, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.relicWeights[characterID]
	return w, ok
}

// mpWeightFromRec 从推荐属性里取某（角色 / 角色+元素）的 7 维评分权重。
//
// 新结构（2026-09-27）：`subStats[].weight` **就是**评分权重（一个词条一个数字 ✓），
// `byElement[元素]` 覆盖对应 statKey；非 7 维的词条（如白值攻击 `FIGHT_PROP_ATTACK`）不影响评分 ✓。
//
// ⚠️ 过渡期规则：**老 payload（带 `score` 块）优先按 score 读** —— 因为老数据的
// `subStats[].weight` 还是"推荐优先级 10 分制"，直接当评分权重用会算错 ✗（迁移完成后可删这段）。
func mpWeightFromRec(characterID int64, rec StatRecommendation, element string) (CharacterRelicWeightMP, bool) {
	if rec.Score != nil {
		if element != "" {
			if dims, ok := rec.Score.ByElement[element]; ok {
				return mpWeightFromDims(characterID, dims), true
			}
			if len(rec.Score.Dims) == 0 {
				return CharacterRelicWeightMP{}, false
			}
		}
		if len(rec.Score.Dims) > 0 {
			return mpWeightFromDims(characterID, rec.Score.Dims), true
		}
		return CharacterRelicWeightMP{}, false
	}
	dims := make(map[string]int, len(rec.SubStats))
	for _, s := range rec.SubStats {
		if s.StatKey != "" {
			dims[s.StatKey] = s.Weight
		}
	}
	if element != "" {
		over, ok := rec.ByElement[element]
		if !ok {
			return CharacterRelicWeightMP{}, false
		}
		for k, v := range over {
			dims[k] = v
		}
	}
	if len(dims) == 0 {
		return CharacterRelicWeightMP{}, false
	}
	return mpWeightFromDims(characterID, dims), true
}

// mpWeightFromDims 把 `FIGHT_PROP_*` → 0~100 的映射转成 7 维权重。
//
// "用 key 对齐"就落在这里 ✓：推荐词条的 statKey 与评分维度是同一批 `FIGHT_PROP_*` ✓
// （cpct ↔ FIGHT_PROP_CRITICAL、cdmg ↔ FIGHT_PROP_CRITICAL_HURT、recharge ↔ FIGHT_PROP_CHARGE_EFFICIENCY…）。
func mpWeightFromDims(characterID int64, dims map[string]int) CharacterRelicWeightMP {
	w := CharacterRelicWeightMP{CharacterID: characterID}
	for prop, v := range dims {
		switch prop {
		case "FIGHT_PROP_HP_PERCENT":
			w.HP = v
		case "FIGHT_PROP_ATTACK_PERCENT":
			w.ATK = v
		case "FIGHT_PROP_DEFENSE_PERCENT":
			w.DEF = v
		case "FIGHT_PROP_CRITICAL":
			w.CPCT = v
		case "FIGHT_PROP_CRITICAL_HURT":
			w.CDMG = v
		case "FIGHT_PROP_ELEMENT_MASTERY":
			w.Mastery = v
		case "FIGHT_PROP_CHARGE_EFFICIENCY":
			w.Recharge = v
		}
	}
	return w
}

// MPWeight 取小助手口径权重（7 维）：先按 (角色, 元素) 查 DB，再退回该角色的通用行；
// 两者都没有时退回**包内嵌种子**（桥接兜底：DB 尚未导入任何东西时口径仍然可用）。
//
// ⚠️ 2026-09-27 起：这份内存映射的**真源是推荐属性 payload 的 `score` 块** ✓
// （`character_relic_weights_mp` 表降级为过渡兜底，见 Load() 里的覆盖逻辑）。
func (s *Service) MPWeight(characterID int64, element string) (CharacterRelicWeightMP, bool) {
	s.mu.RLock()
	m := s.mpWeights
	s.mu.RUnlock()
	if element != "" {
		if w, ok := m[mpWeightKey{ID: characterID, Element: element}]; ok {
			return w, true
		}
	}
	if w, ok := m[mpWeightKey{ID: characterID}]; ok {
		return w, true
	}
	return mpEmbeddedWeight(characterID, element)
}

// RelicWeightRules 按角色取评分权重覆盖规则（返回副本，避免与 reload 竞争）
func (s *Service) RelicWeightRules(characterID int64) []CharacterRelicWeightRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.relicRules[characterID]
	if len(list) == 0 {
		return nil
	}
	out := make([]CharacterRelicWeightRule, len(list))
	copy(out, list)
	return out
}

// travelerFormAliases 旅行者的「形态别名」→ 命中旅行者 ✓（2026-10-14 用户点名 ✓）。
//
// 为什么要这层 ✗：玩家几乎不说"旅行者"✗，只说「**火主 / 冰主**」✓（用户原话：这两个用的人最多 ✓），
// 而这两种写法里都不含"旅行者"三字 ⇒ 纯按名字匹配会**完全命中不到** ✗ ⇒ 攻略卡根本不注入 ✓。
// 命中后用哪一形态，交给 `service.GuideStore` 的形态选择（同样认这几个别名 ✓，火 / 冰优先 ✓）。
var travelerFormAliases = []string{"火主", "冰主", "水主", "草主", "雷主", "风主", "岩主"}

// travelerName 旅行者在 characters 表里的名字（表里每个角色只有一行 ✓；形态是攻略卡侧的维度 ✓）
const travelerName = "旅行者"

// CharactersMentioned 返回提问中「名字被提到」的角色（V2 结构化查表用）。
// 按名字长度倒序，长名优先——否则「雷电将军」会被「将军」这类短名抢先命中；
// 同长度按名字排序，保证结果稳定（map 遍历本身无序）。
func (s *Service) CharactersMentioned(question string) []Character {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil
	}
	q := strings.ToLower(question)

	s.mu.RLock()
	matched := make([]Character, 0, 2)
	for _, c := range s.characters {
		if c.Name != "" && strings.Contains(q, strings.ToLower(c.Name)) {
			matched = append(matched, c)
		}
	}
	// 形态别名：问「火主 / 冰主」时补上旅行者 ✓（本来命中不到 ✗）
	//   ⚠️ 只在别名出现时才补，且**不改变**下面"长名优先"的既有顺序 ✓
	if !containsCharacter(matched, travelerName) {
		aliasHit := false
		for _, a := range travelerFormAliases {
			if strings.Contains(question, a) {
				aliasHit = true
				break
			}
		}
		if aliasHit {
			if c, ok := s.travelerCharacter(); ok {
				matched = append(matched, c)
			}
		}
	}
	// 角色别名（「雷神」→「雷电将军」「帝君」→「钟离」「万叶」→「枫原万叶」✓，2026-10-14 加 ✓）
	//
	// 为什么 ✗：玩家几乎不说全名 ✓，而上面的名字匹配是**精确匹配** ✗ ⇒ 这些叫法命不中任何角色 ✓
	//   ⇒ 攻略卡 / 机制卡 / 词条全都不注入 ✓ ⇒ 回答只能靠模型记忆泛谈 ✓。
	// 真源 = `scripts/data/character_aliases.json` ✓（别在代码里写死 ✗）；只**补**未命中的角色 ✓。
	// ⚠️ map 遍历无序 ✗ ⇒ 先按**别名长度倒序**收集再追加 ✓（「岩王帝君」先于「帝君」✓），保证结果可复现 ✓。
	if len(s.aliases) > 0 {
		type aliasHit struct{ alias, name string }
		hits := make([]aliasHit, 0, 2)
		for alias, name := range s.aliases {
			if strings.Contains(question, alias) && !containsCharacter(matched, name) {
				hits = append(hits, aliasHit{alias, name})
			}
		}
		sort.Slice(hits, func(i, j int) bool {
			li, lj := len([]rune(hits[i].alias)), len([]rune(hits[j].alias))
			if li != lj {
				return li > lj
			}
			return hits[i].alias < hits[j].alias
		})
		for _, h := range hits {
			if containsCharacter(matched, h.name) {
				continue
			}
			for _, c := range s.characters {
				if c.Name == h.name {
					matched = append(matched, c)
					break
				}
			}
		}
	}
	// 错别字容错（2026-10-14 ✓，用户报障：问「伊涅**夫**怎么配队」✗，库里叫「伊涅**芙**」✓
	//   ⇒ 机制卡 / 攻略卡 / 词条**全都没带上** ✓ ⇒ 模型只剩展柜面板、只能老实说"没吃透" ✓）。
	//   ⚠️ 只在"这个名字精确匹配没中"时才试 ✓；一次提问**只补一个**角色 ✗（防连锁误匹配 ✓）。
	if c, asked, ok := s.fuzzyMention(q, matched); ok {
		logger.L().Infow("character matched by fuzzy name", "asked", asked, "name", c.Name)
		matched = append(matched, c)
	}
	// ⚠️ 按名字去重（保留 **ID 最大** 的那个 ✓）—— 2026-10-14 实测到的坑 ✗：
	//   `characters` 表里**有两行都叫「旅行者」**（10000005 / 10000007 ✓，Enka 的那两个 avatarId ✓）
	//   ⇒ ① 问"旅行者"时同一个角色被算两次（词条 2 位 ✗）；
	//     ② 别名补的如果是 map 迭代先撞到的那一行 ✗ ⇒ 有时落到**没有攻略卡的那个 id** 上 ✓
	//        ⇒ 表现为"同一句话有时答得出来、有时答不出来" ✗（攻略卡按 id 查 ⇒ 0 行 ✓）。
	//   保留最大 ID = 与既有数据一致（攻略卡/词条都挂在 10000007 上 ✓），且**结果稳定** ✓。
	matched = dedupByNameKeepMaxID(matched)
	s.mu.RUnlock()

	sort.Slice(matched, func(i, j int) bool {
		li, lj := len([]rune(matched[i].Name)), len([]rune(matched[j].Name))
		if li != lj {
			return li > lj
		}
		return matched[i].Name < matched[j].Name
	})
	return matched
}

// containsCharacter 判断命中列表里是否已有某个角色名（避免重复追加 ✓）
func containsCharacter(list []Character, name string) bool {
	for _, c := range list {
		if c.Name == name {
			return true
		}
	}
	return false
}

// fuzzyMinNameRunes 只有 **≥3 个字**的名字才启用错别字容错 ✓
//
//	两字名差一个字就是另一个名字了 ✗（「胡桃」vs「胡图」）⇒ 宁可不认，也不乱认 ✓
const fuzzyMinNameRunes = 3

// fuzzyStopRunes 虚词：与名字差一个字、且差的就是这几个字时，**不算错别字** ✗
//
//	（否则「那维莱**的**怎么练」会把「那维莱特」认下来 ✓ —— 那是句式不是错字 ✓）
var fuzzyStopRunes = map[rune]bool{
	'的': true, '了': true, '吗': true, '呢': true, '啊': true, '吧': true,
	'嗯': true, '哦': true, '呀': true, '嘛': true, '么': true, '是': true,
}

// fuzzyMention 在提问里找「与某个角色名同长度、只差 1 个字」的写法 ✓（**调用方须持有读锁** ✓）。
//
// 用途（2026-10-14 用户报障 ✓）：中文输入法下「伊涅芙」常被打成「伊涅夫」✗，
//
//	而名字匹配是精确的 ⇒ 机制卡 / 攻略卡 / 词条整条链全断 ✓（模型只剩展柜面板 ✓）。
//
// 三条安全边界（改之前先读 ✓）：
//  1. 只在**精确匹配没命中**这个名字时才试 ✓（写对的名字行为一字不变 ✓）；
//  2. ≥3 个字 ＋ 同长度 ＋ **只差 1 个字** ＋ 差的那个不是虚词 ✓（见 fuzzyStopRunes ✓）；
//  3. `already` 里的名字不参与 ✓（已命中/已被形态别名补过的不会再认 ✗）；
//     调用侧再限制"一次提问只补一个" ✓。
//
// 确定性 ✗：候选按「名字长度倒序 → 字典序」取第一个 ✓ —— map 遍历无序，不能让结果随机 ✓
func (s *Service) fuzzyMention(question string, already []Character) (Character, string, bool) {
	q := []rune(question)
	type cand struct {
		c    Character
		text string
	}
	cands := make([]cand, 0, 4)
	for _, c := range s.characters {
		name := c.Name
		nr := []rune(name)
		if len(nr) < fuzzyMinNameRunes || containsCharacter(already, name) {
			continue
		}
		for i := 0; i+len(nr) <= len(q); i++ {
			diff, diffRune := 0, rune(0)
			for k := range nr {
				if q[i+k] != nr[k] {
					diff++
					diffRune = q[i+k]
					if diff > 1 {
						break
					}
				}
			}
			if diff != 1 || fuzzyStopRunes[diffRune] {
				continue
			}
			cands = append(cands, cand{c: c, text: string(q[i : i+len(nr)])})
			break // 一个名字只取最靠前的一处 ✓
		}
	}
	if len(cands) == 0 {
		return Character{}, "", false
	}
	sort.Slice(cands, func(i, j int) bool {
		li, lj := len([]rune(cands[i].c.Name)), len([]rune(cands[j].c.Name))
		if li != lj {
			return li > lj
		}
		return cands[i].c.Name < cands[j].c.Name
	})
	return cands[0].c, cands[0].text, true
}

// travelerCharacter 取「旅行者」那一行（**调用方必须已持有读锁** ✓）。
// 表里可能有 10000005 / 10000007 两行都叫"旅行者" ⇒ 取 **ID 最大**的那个（与既有数据一致 ✓，且确定 ✓）
func (s *Service) travelerCharacter() (Character, bool) {
	var best Character
	found := false
	for _, c := range s.characters {
		if c.Name != travelerName {
			continue
		}
		if !found || c.ID > best.ID {
			best, found = c, true
		}
	}
	return best, found
}

// dedupByNameKeepMaxID 同名多行只留 ID 最大的那个 ✓
//
// 为什么需要 ✗：`characters` 表里「旅行者」有两行（10000005 / 10000007 ✓）⇒
//
//	不去重的话"提一次旅行者"会被算成两位角色 ✓（注入两份词条、也影响形态选择的数量判断 ✗）。
//
// 顺序：保持传入顺序 ✓（调用方随后会按"长名优先"排序 ✓，这里只做替换 ✓）。
func dedupByNameKeepMaxID(list []Character) []Character {
	best := make(map[string]Character, len(list))
	for _, c := range list {
		if cur, ok := best[c.Name]; !ok || c.ID > cur.ID {
			best[c.Name] = c
		}
	}
	out := make([]Character, 0, len(best))
	seen := make(map[string]bool, len(best))
	for _, c := range list {
		if seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		out = append(out, best[c.Name])
	}
	return out
}

// RelicSetsMentioned 返回提问中「名字被提到」的圣遗物套装（V2.2 结构化查表用）。
// 与 CharactersMentioned 同一套策略：长名优先、同长按名字排序（map 遍历无序，结果要稳定）。
func (s *Service) RelicSetsMentioned(question string) []RelicSet {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil
	}
	q := strings.ToLower(question)

	s.mu.RLock()
	matched := make([]RelicSet, 0, 2)
	for _, rs := range s.relicSets {
		if rs.Name != "" && strings.Contains(q, strings.ToLower(rs.Name)) {
			matched = append(matched, rs)
		}
	}
	s.mu.RUnlock()

	sort.Slice(matched, func(i, j int) bool {
		li, lj := len([]rune(matched[i].Name)), len([]rune(matched[j].Name))
		if li != lj {
			return li > lj
		}
		return matched[i].Name < matched[j].Name
	})
	return matched
}

// ReportMissing 异步记录未命中的 ID（补录依据）
func (s *Service) ReportMissing(kind, id string) {
	if s.db == nil || id == "" {
		return
	}
	go func() {
		m := Missing{Kind: kind, AssetID: id, FirstSeen: time.Now(), HitCount: 1}
		err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "kind"}, {Name: "asset_id"}},
			DoUpdates: clause.Set{{
				Column: clause.Column{Name: "hit_count"},
				Value:  gorm.Expr("hit_count + 1"),
			}},
		}).Create(&m).Error
		if err != nil {
			logger.L().Warnw("report missing asset failed", "kind", kind, "id", id, "err", err)
		}
	}()
}

// MissingList 缺漏清单（按命中次数倒序）
func (s *Service) MissingList(limit int) ([]Missing, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var list []Missing
	err := s.db.Order("hit_count DESC").Limit(limit).Find(&list).Error
	return list, err
}
