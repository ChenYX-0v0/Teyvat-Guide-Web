package service

// PlayerService 玩家查询编排：Enka 拉取 → 映射翻译 → 结构化输出
// 查询成功后异步将档案快照（JSON）落库至 player_profile_snapshots 表

import (
	"context"
	"encoding/json"
	"strings"
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
// → 圣遗物部位枚举；中文部位名从 relic_slots 表取，不硬编码。
// （表名 2026-09-27 校正：文档里的 `asset_relic_slots` 是早期设计名 ✗，实际无 `asset_` 前缀 ✓）
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
	// guides 角色攻略卡（2026-10-13，阶段②）：**直查库**、不进 asset 的内存缓存 ✓
	guides *GuideStore
	// mechanics 角色机制卡（2026-10-13，阶段② TODO-20）：同口径，直查库 ✓
	mechanics *MechanicStore
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
		guides:     NewGuideStore(db),
		mechanics:  NewMechanicStore(db),
	}
}

// Query 查询玩家档案
// 默认先查本地快照（player_profile_snapshots 表），命中直接返回；未命中或 forceRefresh 时才调 Enka API
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

// CollectCharacters 采集本轮展柜角色到「已收录角色」表（**只增 / 只改，永不删** ✓，2026-10-11）。
//
// 调用范围由 handler 保证：**仅当查询的 UID 正是当前用户绑定的 UID** 时才调 ✓
//
//	—— 否则任何人都能刷任意 UID 把表撑大 ✗，也等于替"别人的号"存了数据 ✗。
//
// 写两笔：① upsert 本轮见到的角色 ✓（`first_seen_at` 冲突时**不覆盖** ✓）；
// ② 把不在本轮展柜里的行 `in_showcase` 置 false ✓ —— **这是标记，不是删除** ✓，
// 用户换回展柜时那行还在 ✓（这正是"换个展柜就把所有角色导进来"的实现方式 ✓）。
// 失败只记日志：采集是**附带收益** ✓，绝不该让查询本身失败 ✗（与快照写入同一口径 ✓）。
func (s *PlayerService) CollectCharacters(uid string, cards []dto.CharacterCard) {
	if uid == "" || len(cards) == 0 {
		return
	}
	now := time.Now()
	rows := make([]model.PlayerCharacter, 0, len(cards))
	keep := make([]int64, 0, len(cards))
	for _, c := range cards {
		raw, err := json.Marshal(c)
		if err != nil {
			logger.L().Warnw("marshal collected character failed", "uid", uid, "id", c.ID, "err", err)
			continue
		}
		rows = append(rows, model.PlayerCharacter{
			UID:         uid,
			CharacterID: c.ID,
			Card:        string(raw),
			InShowcase:  true,
			FirstSeenAt: now, // ⚠️ 冲突时不覆盖 ⇒ 这个值只在**首次**落库时生效 ✓
			LastSeenAt:  now,
		})
		keep = append(keep, c.ID)
	}
	if len(rows) == 0 {
		return
	}
	if err := s.repo.UpsertCharacters(rows); err != nil {
		logger.L().Warnw("upsert collected characters failed", "uid", uid, "err", err)
		return
	}
	if err := s.repo.MarkCharactersOutOfShowcase(uid, keep); err != nil {
		logger.L().Warnw("mark characters out of showcase failed", "uid", uid, "err", err)
		return
	}
	logger.L().Infow("collected showcase characters", "uid", uid, "seen", len(rows))
}

// Characters 读取该 UID 已收录的全部角色（展柜中的在前 ✓、同组内按最近见到倒序 ✓）。
// 表未启用 / 无记录时返回**空集合**（不是错误 ✓）—— 前端据此显示"还没有收录记录" ✓。
func (s *PlayerService) Characters(uid string) (*dto.CharacterCollection, error) {
	rows, err := s.repo.ListCharacters(uid)
	if err != nil {
		return nil, err
	}
	out := &dto.CharacterCollection{UID: uid, Characters: make([]dto.CollectedCharacter, 0, len(rows))}
	for _, r := range rows {
		var card dto.CharacterCard
		if err := json.Unmarshal([]byte(r.Card), &card); err != nil {
			// 单行解析失败只跳过这一行（老数据 / 结构变更），不影响整份列表 ✓
			logger.L().Warnw("unmarshal collected character failed", "uid", uid, "id", r.CharacterID, "err", err)
			continue
		}
		out.Characters = append(out.Characters, dto.CollectedCharacter{
			CharacterCard: card,
			InShowcase:    r.InShowcase,
			FirstSeenAt:   r.FirstSeenAt,
			LastSeenAt:    r.LastSeenAt,
		})
		if r.InShowcase {
			out.ShowcaseCount++
		}
	}
	out.Total = len(out.Characters)
	return out, nil
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

// GuidesForQuestion 返回提问中命中角色的攻略卡文本（阶段②，2026-10-13，见 ai/07 TODO-18）。
// 解决的是"配队思路 / 毕业线 / 天赋顺序"这三类**库里一条都没有**的问题 ✗（此前模型只能
// "没有查到"或凭记忆泛谈 ✓）。与 StatWeightsForQuestion 的差异只有一处：
// **直查库**（不走 asset 的内存缓存 ✓）⇒ 后台改完立即生效 ✓。
// dropTeam：已带机制卡的角色名 ⇒ 这些角色的「配队思路」结论不再注入 ✓（见 character_mechanic.go）
func (s *PlayerService) GuidesForQuestion(question string, dropTeam map[string]bool) string {
	if s == nil || s.guides == nil {
		return ""
	}
	return s.guides.ForQuestion(s.assets, question, dropTeam)
}

// MechanicsForQuestion 返回提问中命中角色的**机制事实**卡与命中的全局规则（阶段②，TODO-20）。
// 与 GuidesForQuestion 的分工：机制卡给事实（让它自己推 ✓），攻略卡给结论（毕业线 / 天赋 ✓）。
func (s *PlayerService) MechanicsForQuestion(question string) MechanicHit {
	if s == nil || s.mechanics == nil {
		return MechanicHit{}
	}
	return s.mechanics.ForQuestion(s.assets, question)
}

// SummaryStats 组装摘要时的计数（只为日志与排障 ✓，不参与业务判断 ✓）
type SummaryStats struct {
	MechanicCards int
	Rules         []string
	GuideCards    int
	Refs          int
	Sets          int
}

// SummaryForQuestion 组装发给 AI 服务的 `context.summary` 文本 —— **唯一出口** ✓。
//
// 为什么不分散在各处拼 ✗：这条链有顺序要求，且**离线工具要复刻同一条链**（评测器要拿
// 一模一样的注入文本去打 AI 服务 ✓，见 cmd/mechanicsimport 的 -questions/-out ✓）。
// 集中在这里后，线上与评测走的是**同一段代码** ✓（2026-10-13 ✓）。
//
// 顺序 = 优先级（越靠前越不容易被下游预算裁掉 ✓）：
//
//	① 【角色机制】事实 → ② 【角色攻略】结论 → ③ 【游戏资料】权威数值
//
// 匿名用户也能拿到这三块（公开数据 ✓，与登录态无关 ✓）。
func (s *PlayerService) SummaryForQuestion(question string) (string, SummaryStats) {
	stats := SummaryStats{}
	if s == nil || s.assets == nil {
		return "", stats
	}

	// ① 机制卡：先算，因为攻略卡要据此**去掉配队思路结论** ✓
	//   ⚠️ 阶段③（2026-10-13 铺全量后）起：**有机制卡就丢配队思路** ✗ —— 包括 draft 卡 ✓。
	//   理由见 ai/07 TODO-20 护栏 3：用户要的不是"背某页列的队"✗，而是"按事实自己推"✓；
	//   自动卡虽然只是初稿 ✓，但它给的是**逐字校验过的原句** ✓，比反向索引拼出来的"常被编入…"更可靠 ✓。
	hit := s.MechanicsForQuestion(question)
	stats.MechanicCards = len(hit.Characters)
	stats.Rules = hit.Rules

	dropTeam := make(map[string]bool, len(hit.Characters))
	for _, name := range hit.Characters {
		dropTeam[name] = true
	}

	refs := s.StatWeightsForQuestion(question)
	sets := s.RelicSetEffectsForQuestion(question)
	stats.Refs, stats.Sets = len(refs), len(sets)
	assetText := BuildAssetSummary(refs, sets)

	// ② 攻略卡（已带机制卡的角色不含配队思路 ✓）
	guide := s.GuidesForQuestion(question, dropTeam)
	stats.GuideCards = strings.Count(guide, "【角色攻略】")

	parts := make([]string, 0, 3)
	for _, block := range []string{hit.Text, guide, assetText} {
		if strings.TrimSpace(block) != "" {
			parts = append(parts, block)
		}
	}
	return strings.Join(parts, "\n"), stats
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

// DefaultGenshinServer 绑定时的默认区服 —— 显式绑定（BindUID）与自动绑定（AutoBind）**共用一份** ✓，
// 免得两处各写一遍字面量后漂移 ✗。
const DefaultGenshinServer = "cn_gf01"

// BindUID 将 UID 绑定到用户：绑定前先通过 Enka 实查验证 UID 存在且档案公开
func (s *PlayerService) BindUID(ctx context.Context, userID uint, uid, server string) error {
	if _, err := s.enka.FetchProfile(ctx, uid); err != nil {
		return err // UID 不存在 / 未公开 / 上游异常，均拒绝绑定
	}
	return s.userRepo.UpdateBinding(userID, uid, server)
}

// AutoBind **查询成功后自动绑定**该 UID（2026-10-12 用户口径 A ✓）：省掉"查完再点一次绑定"那一步 ✓。
//
// 与 BindUID 的区别：**不再向 Enka 复查一次** ✓ —— 调用点（`handler/genshin.go` 的 `Player`）
// 刚刚成功取回了该 UID 的档案，已经证明"UID 真实存在且档案公开" ✓；再查一次纯属多打一次上游 ✗。
// 也正因如此，调用点必须放在**档案取回来之后** ✓ —— 查一个不存在 / 未公开的 UID 绝不能把账号绑过去 ✗。
//
// 幂等与换绑（口径 A 的代价，用户已确认 ✓）：
//   - 已绑的是**同一个** UID ⇒ **不写库**，返回 changed=false ✓（每次查询都写一遍纯属噪音 ✗）；
//   - 绑的是**别的** UID ⇒ **改绑** ✓；
//   - `server` 沿用原有值 ✓（换绑时别顺手把它清空 ✗），从未绑过则用 DefaultGenshinServer ✓；
//   - `userID == 0`（未登录）或 uid 为空 ⇒ 直接返回 ✓ —— **匿名查询的行为完全不变** ✓。
func (s *PlayerService) AutoBind(userID uint, uid string) (changed bool, err error) {
	if userID == 0 || uid == "" {
		return false, nil
	}
	curUID, curServer, err := s.Binding(userID)
	if err != nil {
		return false, err
	}
	if curUID == uid {
		return false, nil
	}
	server := curServer
	if server == "" {
		server = DefaultGenshinServer
	}
	if err := s.userRepo.UpdateBinding(userID, uid, server); err != nil {
		return false, err
	}
	return true, nil
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

// ContextStatus 给聊天页用的轻量状态：**派蒙现在能不能看到你的角色**（2026-10-12 ✓）。
//
// 口径（别改成查上游 ✗）：只读本地 —— 绑定 ✓ + 档案快照 ✓ + 已收录角色池 ✓。
// 之所以算上"池子"：聊天侧有兜底（上游失败 / 展柜为空时用池子拼档案 ✓），
// 所以"有池子"就意味着**聊天能带上角色信息** ✓ —— 状态与真实行为保持一致 ✓。
func (s *PlayerService) ContextStatus(userID uint) dto.PlayerContextStatus {
	st := dto.PlayerContextStatus{Reason: "not_bound"}
	if userID == 0 {
		return st
	}
	uid, _, err := s.Binding(userID)
	if err != nil || uid == "" {
		return st
	}
	st.Bound = true
	st.UID = uid

	if p, err := s.cachedProfile(uid); err == nil && p != nil && len(p.Characters) > 0 {
		st.HasSnapshot = true
	}
	if col, err := s.Characters(uid); err == nil && col != nil {
		st.CollectedCount = len(col.Characters)
	}
	st.Ready = st.HasSnapshot || st.CollectedCount > 0
	if !st.Ready {
		st.Reason = "no_profile"
	} else {
		st.Reason = ""
	}
	return st
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
