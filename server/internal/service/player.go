package service

// PlayerService 玩家查询编排：Enka 拉取 → 映射翻译 → 结构化输出
// 查询成功后异步将档案快照（JSON）落库至 player_profiles 表

import (
	"context"
	"encoding/json"
	"time"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
	"teyvat-guide/server/internal/service/asset"
	"teyvat-guide/server/internal/service/enka"

	"gorm.io/gorm"
)

// snapshotTTL 档案快照有效期：超过则重新拉取上游
// （Enka 慢且有限流，故不每次直查；但面板过旧会误导养成建议，故设 10 分钟上限）
const snapshotTTL = 10 * time.Minute

// statSlotEnums 推荐词条 payload 的部位 key（契约见 docs/04-API接口文档.md §6.2）
// → 圣遗物部位枚举；中文部位名从 asset_relic_slots 表取，不硬编码。
var statSlotEnums = []struct{ key, pos string }{
	{"sand", "EQUIP_SHOES"},    // 时之沙
	{"goblet", "EQUIP_RING"},   // 空之杯
	{"circlet", "EQUIP_DRESS"}, // 理之冠
}

// CharacterStatRef 一个角色的权威资料（V2 结构化查表用）。
// SubStats 已翻译成中文并按权重降序；SubStats/MainStats 同时为空表示「本服暂无该角色的资料」。
type CharacterStatRef struct {
	Name      string
	SubStats  []dto.StatWeightEntry
	MainStats []StatSlotRef // 按沙/杯/冠固定顺序，保证渲染文本可复现
}

// RelicSetEffectRef 套装某个件数的效果（Piece = 2 或 4）
type RelicSetEffectRef struct {
	Piece  int
	Effect string
}

// RelicSetRef 一个圣遗物套装的权威效果（V2.2 结构化查表用，2026-09-15 新增）。
// Effects 为空表示「名字命中、但本服没有这个套装的效果资料」。
type RelicSetRef struct {
	Name    string
	Effects []RelicSetEffectRef // 按件数升序（2、4）
}

// StatSlotRef 一个部位的主词条推荐
type StatSlotRef struct {
	Slot string // 中文部位名（取自映射表）
	Stat string // 推荐主属性
}

type PlayerService struct {
	assets     *asset.Service
	enka       *enka.Client
	translator *enka.Translator
	repo       *repository.PlayerRepository
	userRepo   *repository.UserRepository
}

// NewPlayerService db 用于映射数据持久层与快照存储（nil 时翻译走 fallback、快照不落库）
func NewPlayerService(db *gorm.DB) *PlayerService {
	assets := asset.New(db)
	if err := assets.Load(); err != nil {
		logger.L().Warnw("asset maps load failed, fallback names will be used", "err", err)
	}
	return &PlayerService{
		assets:     assets,
		enka:       enka.NewClient(),
		translator: enka.NewTranslator(assets),
		repo:       repository.NewPlayerRepository(),
		userRepo:   repository.NewUserRepository(),
	}
}

// Query 查询玩家档案
// 默认先查本地快照（player_profiles 表），命中直接返回；未命中或 forceRefresh 时才调 Enka API
// 查询成功后异步保存快照
func (s *PlayerService) Query(ctx context.Context, uid string, forceRefresh bool) (*dto.PlayerProfile, error) {
	if !forceRefresh {
		if profile, err := s.cachedProfile(uid); err == nil && profile != nil {
			logger.L().Infow("player profile served from cache", "uid", uid)
			return profile, nil
		}
	}

	profile, err := s.enka.FetchProfile(ctx, uid)
	if err != nil {
		return nil, err
	}
	result := s.translator.TranslateProfile(profile)
	s.saveSnapshot(result)
	return result, nil
}

// cachedProfile 从快照表恢复档案（表未启用 / 无记录 / 解析失败均返回 nil）
// 超过 snapshotTTL 视为过期，返回 nil 触发向上游重新拉取（对 AI 上下文而言，旧面板会误导回答）
func (s *PlayerService) cachedProfile(uid string) (*dto.PlayerProfile, error) {
	snapshot, err := s.repo.GetSnapshot(uid)
	if err != nil || snapshot == nil {
		return nil, err
	}
	if age := time.Since(snapshot.UpdatedAt); age > snapshotTTL {
		logger.L().Infow("player snapshot expired, refetch upstream",
			"uid", uid, "age", age.Truncate(time.Second).String())
		return nil, nil
	}
	var profile dto.PlayerProfile
	if err := json.Unmarshal([]byte(snapshot.Profile), &profile); err != nil {
		logger.L().Warnw("unmarshal cached profile failed", "uid", uid, "err", err)
		return nil, err
	}
	return &profile, nil
}

// saveSnapshot 异步将档案 JSON 落库（uid 主键 upsert，失败仅记日志不阻塞查询）
func (s *PlayerService) saveSnapshot(p *dto.PlayerProfile) {
	raw, err := json.Marshal(p)
	if err != nil {
		logger.L().Warnw("marshal player profile failed", "uid", p.UID, "err", err)
		return
	}
	if err := s.repo.SaveSnapshot(p.UID, string(raw)); err != nil {
		logger.L().Warnw("save player snapshot failed", "uid", p.UID, "err", err)
		return
	}
	logger.L().Infow("player snapshot saved", "uid", p.UID, "bytes", len(raw))
}

// Snapshot 读取已落库的档案快照（不发起上游查询）
func (s *PlayerService) Snapshot(uid string) (*model.PlayerProfileSnapshot, error) {
	return s.repo.GetSnapshot(uid)
}

// ImportAssets 批量导入映射数据
func (s *PlayerService) ImportAssets(p asset.ImportPayload) error {
	return s.assets.Import(p)
}

// StatWeightsForQuestion 返回提问中命中角色的权威推荐词条（V2.1 结构化查表）。
// 只读公开资产数据，与登录态无关；命中但库里没有数据的角色也会返回（两个字段均为空），
// 让上层能明确告诉模型「本服没有这份资料」，而不是让它凭记忆编数值（V2.3）。
func (s *PlayerService) StatWeightsForQuestion(question string) []CharacterStatRef {
	chars := s.assets.CharactersMentioned(question)
	if len(chars) == 0 {
		return nil
	}

	refs := make([]CharacterStatRef, 0, len(chars))
	for _, c := range chars {
		ref := CharacterStatRef{Name: c.Name}
		if rec, ok := s.assets.StatRecommendation(c.ID); ok {
			if tr := s.translator.TranslateStatRecommendation(rec); tr != nil {
				ref.SubStats = tr.SubStats
				ref.MainStats = s.mainStatLabels(tr.MainStats)
			}
		}
		refs = append(refs, ref)
	}
	return refs
}

// RelicSetEffectsForQuestion 返回提问中命中套装的 2/4 件效果（V2.2 结构化查表，2026-09-15）。
// 与 StatWeightsForQuestion 同构：只读公开资产数据、与登录态无关；名字命中但库里没有效果的套装
// 也会返回（Effects 为空），让上层显式告诉模型「本服暂无资料」，而不是凭记忆描述效果（V2.3 同款保险）。
func (s *PlayerService) RelicSetEffectsForQuestion(question string) []RelicSetRef {
	sets := s.assets.RelicSetsMentioned(question)
	if len(sets) == 0 {
		return nil
	}

	refs := make([]RelicSetRef, 0, len(sets))
	for _, rs := range sets {
		ref := RelicSetRef{Name: rs.Name}
		for _, e := range s.assets.RelicSetEffects(rs.SetID) {
			if e.Effect == "" {
				continue
			}
			ref.Effects = append(ref.Effects, RelicSetEffectRef{Piece: e.Piece, Effect: e.Effect})
		}
		refs = append(refs, ref)
	}
	return refs
}

// mainStatLabels 把 payload 的部位 key 换成中文部位名（中文名取自映射表），
// 按沙/杯/冠固定顺序输出，避免 map 遍历顺序让同一问题每次渲染出不同文本
func (s *PlayerService) mainStatLabels(main map[string]string) []StatSlotRef {
	if len(main) == 0 {
		return nil
	}
	out := make([]StatSlotRef, 0, len(main))
	for _, slot := range statSlotEnums {
		if v, ok := main[slot.key]; ok && v != "" {
			out = append(out, StatSlotRef{Slot: s.assets.RelicSlotName(slot.pos), Stat: v})
		}
	}
	return out
}

// ---------- 账号 ↔ 游戏 UID 绑定（一对一） ----------

// BindUID 将 UID 绑定到用户：绑定前先通过 Enka 实查验证 UID 存在且档案公开
func (s *PlayerService) BindUID(ctx context.Context, userID uint, uid, server string) error {
	if _, err := s.enka.FetchProfile(ctx, uid); err != nil {
		return err // UID 不存在 / 未公开 / 上游异常，均拒绝绑定
	}
	return s.userRepo.UpdateBinding(userID, uid, server)
}

// Binding 查询用户绑定的 UID（未绑定时返回空串）
func (s *PlayerService) Binding(userID uint) (uid, server string, err error) {
	u, err := s.userRepo.FindByID(userID)
	if err != nil || u == nil {
		return "", "", err
	}
	return u.GenshinUID, u.GenshinServer, nil
}

// Unbind 解除绑定
func (s *PlayerService) Unbind(userID uint) error {
	return s.userRepo.UpdateBinding(userID, "", "")
}

// MissingAssets 缺漏清单
func (s *PlayerService) MissingAssets(limit int) ([]asset.Missing, error) {
	return s.assets.MissingList(limit)
}

// ReloadAssets 手动触发映射重载；purgeSnapshots 为真时同时清空档案快照
// （快照里存的是旧翻译结果，映射变更后不清会成为脏数据）
func (s *PlayerService) ReloadAssets(purgeSnapshots bool) error {
	if err := s.assets.Reload(); err != nil {
		return err
	}
	if purgeSnapshots {
		return s.PurgeSnapshots()
	}
	return nil
}

// PurgeSnapshots 清空全部档案快照
func (s *PlayerService) PurgeSnapshots() error {
	if err := s.repo.ClearSnapshots(); err != nil {
		logger.L().Warnw("clear player snapshots failed", "err", err)
		return err
	}
	logger.L().Infow("player snapshots cleared")
	return nil
}
