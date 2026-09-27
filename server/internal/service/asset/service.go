package asset

// AssetService 映射数据服务
// - 启动时从 MySQL 全量加载进内存，翻译零 SQL
// - 未命中异步写入 asset_missing 供补录
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

	mu           sync.RWMutex
	characters   map[int64]Character
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
		// 词条名 → PropID：**同名冲突时优先保留"有数字枚举"的那个**。
		// 反例（2026-09-26 踩到）：FIGHT_PROP_FIRE_ADD 与 FIGHT_PROP_FIRE_ADD_HURT 都叫「火元素伤害加成」
		// —— 面板属性行要用数字枚举回查数值，若选中没有枚举的 _ADD_HURT，整行会被跳过，
		// 表现为「角色面板里 元素伤害加成 整项消失」。顺序敏感：所以先建 numByFight 再建本表。
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

	s.mu.Lock()
	s.characters = charMap
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

// NumIDByPropID PropID → 数字枚举（如 FIGHT_PROP_WATER_ADD → "45"）
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
