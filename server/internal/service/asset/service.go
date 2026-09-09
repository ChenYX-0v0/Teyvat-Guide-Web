package asset

// AssetService 映射数据服务
// - 启动时从 MySQL 全量加载进内存，翻译零 SQL
// - 未命中异步写入 asset_missing 供补录
// - Import 后自动 reload，立即生效无需重启

import (
	"encoding/json"
	"errors"
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
	weapons    map[int64]Weapon
	relicSets  map[int]RelicSet
	texts      map[int64]string
	props        map[string]PropMap
	propIDByName map[string]string // 词条名 → PropID
	fightProps   map[string]string // 数字枚举 → PropID（正向）
	numByFight   map[string]string // PropID → 数字枚举（反向）
	affixes      map[int64]Affix
	statRecs   map[int64]StatRecommendation
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
	var statRecRows []CharacterStatWeights
	if err := s.db.Find(&statRecRows).Error; err != nil {
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
	propIDByName := make(map[string]string, len(props))
	for _, p := range props {
		propMap[p.PropID] = p
		propIDByName[p.Name] = p.PropID
	}
	fightByNum := make(map[string]string, len(fightProps))
	numByFight := make(map[string]string, len(fightProps))
	for _, f := range fightProps {
		fightByNum[f.NumID] = f.PropID
		numByFight[f.PropID] = f.NumID
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

	s.mu.Lock()
	s.characters = charMap
	s.weapons = weaponMap
	s.relicSets = setMap
	s.texts = textMap
	s.props = propMap
	s.propIDByName = propIDByName
	s.fightProps = fightByNum
	s.numByFight = numByFight
	s.affixes = affixMap
	s.statRecs = recMap
	s.mu.Unlock()

	logger.L().Infow("asset maps loaded",
		"characters", len(chars), "weapons", len(weapons),
		"relicSets", len(sets), "texts", len(texts), "props", len(props),
		"fightProps", len(fightProps), "affixes", len(affixes), "statRecs", len(recMap))
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
