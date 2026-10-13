package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"

	"github.com/google/uuid"
)

const (
	historyLimit       = 20   // 历史条数上限
	historyTokenBudget = 1200 // 历史 token 预算（中文 1 字符 ≈ 1 token 粗估，仅用于裁剪不用于计费）
)

// aiDegradeReply AI 服务不可用时的兜底回复（保持会话连续，不让请求失败）
const aiDegradeReply = "派蒙现在有点晕乎乎的，暂时联系不上她的脑子……旅行者稍后再问我一次好不好？"

// AIStreamInterruptedReply 流式回复中途失败时的提示。已脱敏，禁止把上游细节透给前端。
const AIStreamInterruptedReply = "派蒙说着说着突然走神了……旅行者再问我一次好不好？"

// ChatService 对话业务编排
type ChatService struct {
	cfg     config.AIConfig
	ai      AIClient
	repo    *repository.ChatRepository
	players *PlayerService // 可为 nil：无玩家查询能力时不做上下文推送
	quota   *QuotaService  // 可为 nil（单测构造）：为 nil 时不做配额与并发限制
}

func NewChatService(cfg config.AIConfig, players *PlayerService, quota *QuotaService) *ChatService {
	return &ChatService{
		cfg:     cfg,
		ai:      NewAIClient(cfg),
		repo:    repository.NewChatRepository(),
		players: players,
		quota:   quota,
	}
}

// acquireQuota 配额与并发占位（未注入 QuotaService 时直接放行，便于单测构造）
func (s *ChatService) acquireQuota(userID uint) (func(), error) {
	if s.quota == nil {
		return func() {}, nil
	}
	return s.quota.Acquire(userID)
}

// contextAssembly 一次提问组装出的业务上下文，附带"是否注入过"的标记
// （标记用于后台观测 V1 玩家档案 / V2 资产资料的实际命中情况）
type contextAssembly struct {
	rc        *RequestContext
	hasPlayer bool
	hasAsset  bool
}

// assistantRecord 助手回复的落库载荷
type assistantRecord struct {
	Content   string
	Usage     AIUsage
	Degraded  bool
	ElapsedMs int
}

// Ask 处理一次提问（非流式）
// userID 为 0 表示匿名（不推送玩家上下文）；非 0 且已绑定 UID 时注入玩家档案摘要
func (s *ChatService) Ask(ctx context.Context, userID uint, sessionID, question string) (string, string, int, int64, error) {
	start := time.Now()

	// 配额与并发检查必须在**建会话之前**：超限直接返回 429，不产生任何会话/消息（docs/04 §2.6）
	release, err := s.acquireQuota(userID)
	if err != nil {
		return "", "", 0, 0, err
	}
	defer release()

	sessionID, userMsgID, history, asm, err := s.prepare(ctx, userID, sessionID, question)
	if err != nil {
		return "", "", 0, 0, err
	}

	reply, usage, err := s.ai.Complete(ctx, s.cfg.System, history, question, asm.rc)
	rec := assistantRecord{Content: reply, Usage: usage}
	if err != nil {
		// 上游（AI 服务 / 模型）故障：只记日志，不向前端暴露内部细节，返回可读降级文案
		logger.L().Errorw("ai complete failed",
			"err", err,
			"session", sessionID,
			"provider", s.cfg.Provider,
		)
		rec = assistantRecord{Content: aiDegradeReply, Degraded: true}
		// 降级回复同样落库（保持会话连续），标记便于事后排查上游可用性
		logger.L().Infow("chat degraded, fallback reply persisted", "session", sessionID)
	}
	rec.ElapsedMs = int(time.Since(start).Milliseconds())
	s.persistExchange(sessionID, userMsgID, asm, rec)

	return sessionID, rec.Content, rec.Usage.Tokens, time.Since(start).Milliseconds(), nil
}

// AskStream 处理一次提问（流式）
// 增量文本通过 onDelta 逐段回调（由 Handler 写成 SSE 帧），会话建立时先回调 onSession。
// 返回会话 ID、真实 token 用量与耗时；上游中途失败时：
//   - 尚未产出任何内容 → 与同步链路一致地给出可读降级文案（err 为 nil，HTTP 仍是 200）
//   - 已有部分内容 → 返回 err（文案已脱敏），由 Handler 以 error 帧结束
func (s *ChatService) AskStream(
	ctx context.Context,
	userID uint,
	sessionID, question string,
	onSession func(sessionID string) error,
	onDelta func(text string) error,
) (string, int, int64, error) {
	start := time.Now()

	// 配额与并发检查在**任何 SSE 帧写出之前**（Handler 因此还能返回 429 JSON，见 docs/04 §2.6）
	release, err := s.acquireQuota(userID)
	if err != nil {
		return "", 0, 0, err
	}
	defer release()

	sessionID, userMsgID, history, asm, err := s.prepare(ctx, userID, sessionID, question)
	if err != nil {
		return "", 0, 0, err
	}
	// 会话 ID 先于任何增量下发，前端在流中断时也能保留会话归属
	if onSession != nil {
		if err := onSession(sessionID); err != nil {
			return sessionID, 0, 0, err
		}
	}

	var buf strings.Builder
	produced := false
	emit := func(text string) error {
		if text == "" {
			return nil
		}
		produced = true
		buf.WriteString(text)
		return onDelta(text)
	}

	usage, streamErr := s.ai.CompleteStream(ctx, s.cfg.System, history, question, asm.rc, emit)
	rec := assistantRecord{Content: buf.String(), Usage: usage}
	if streamErr != nil {
		logger.L().Errorw("ai stream failed",
			"err", streamErr,
			"session", sessionID,
			"provider", s.cfg.Provider,
			"produced", produced,
		)
		switch {
		case produced:
			// 已吐出部分内容：不再追加，交由 Handler 以 error 帧提示（不泄漏上游细节）
			rec.Degraded = true
		case ctx.Err() != nil:
			// 客户端已断开：没必要再补写降级文案
		default:
			// 一个增量都没发出：按同步链路的降级策略给出可读文案，用户仍能看到回复
			rec = assistantRecord{Content: aiDegradeReply, Degraded: true}
			if err := emit(aiDegradeReply); err == nil {
				logger.L().Infow("chat stream degraded, fallback reply persisted", "session", sessionID)
				streamErr = nil
			}
		}
	}
	rec.ElapsedMs = int(time.Since(start).Milliseconds())
	s.persistExchange(sessionID, userMsgID, asm, rec)

	return sessionID, rec.Usage.Tokens, time.Since(start).Milliseconds(), streamErr
}

// prepare 提问前的公共编排：建会话 → 落库用户消息 → 组装历史与业务上下文。
// 第二个返回值是本轮提问的消息 ID，用于回写无关问题判定（落库失败时为 0）。
func (s *ChatService) prepare(ctx context.Context, userID uint, sessionID, question string) (string, uint, []Message, contextAssembly, error) {
	question = trim(question)
	if question == "" {
		return "", 0, nil, contextAssembly{}, errors.New("问题不能为空")
	}

	// 归属校验：非本人（或已不存在、已删除）的 sessionId 一律当作新会话——
	// 既避免"拿到别人的 id 就能续聊"，也避免已删除的会话被续聊复活（口径见 docs/04 §2.4）。
	if sessionID != "" && !s.ownsSession(sessionID, userID) {
		sessionID = ""
	}

	if sessionID == "" {
		sessionID = uuid.NewString()
		if err := s.repo.CreateSession(&model.ChatSession{
			ID:        sessionID,
			Title:     titleOf(question),
			UserID:    userID, // 0 曾表示匿名会话；2026-09-14 起匿名已不能提问，仅存量数据
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			logger.L().Errorw("create session failed", "session", sessionID, "err", err)
		}
	}

	userMsg := &model.ChatMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   question,
		CreatedAt: time.Now(),
	}
	if err := s.repo.AppendMessage(userMsg); err != nil {
		logger.L().Errorw("append user message failed", "session", sessionID, "err", err)
	}

	recent, err := s.repo.ListRecentMessages(sessionID, historyLimit)
	if err != nil {
		return sessionID, userMsg.ID, nil, contextAssembly{}, err
	}

	return sessionID, userMsg.ID, buildHistory(recent, question), s.buildContext(ctx, userID, question), nil
}

// ownsSession 校验会话是否存在、归属该用户、且未被删除
// （userID = 0 即匿名会话，其 user_id 也是 0）。
// 查库失败时按"不认识"处理（返回 false）：宁可新开一个会话，也不要误续别人的会话。
// 已软删除的会话也返回 false → 用户续聊会开一个新会话，不会让删掉的会话"复活"。
func (s *ChatService) ownsSession(sessionID string, userID uint) bool {
	session, err := s.repo.FindSession(sessionID)
	if err != nil {
		logger.L().Warnw("find session failed", "session", sessionID, "err", err)
		return false
	}
	return session != nil && session.UserID == userID && session.DeletedAt == nil
}

// persistExchange 落库助手回复与会话观测标记（失败只记日志，不影响本次回答）
// userMsgID：本轮提问的消息 ID，用于回写无关问题判定（见 shared-docs/05 §3.3）
func (s *ChatService) persistExchange(sessionID string, userMsgID uint, asm contextAssembly, rec assistantRecord) {
	if err := s.repo.AppendMessage(&model.ChatMessage{
		SessionID:    sessionID,
		Role:         "assistant",
		Content:      rec.Content,
		Tokens:       rec.Usage.Tokens,
		TokensInput:  rec.Usage.InputTokens,
		TokensOutput: rec.Usage.OutputTokens,
		TokensCached: rec.Usage.CachedTokens,
		Model:        rec.Usage.Model,
		Profile:      rec.Usage.Profile,
		Degraded:     rec.Degraded,
		ElapsedMs:    rec.ElapsedMs,
		CreatedAt:    time.Now(),
	}); err != nil {
		logger.L().Errorw("append assistant message failed", "session", sessionID, "err", err)
	}

	// 无关问题判定回写到**用户提问**那条消息上（OffTopic 为 nil 表示未判定 → 保持 NULL）
	if err := s.repo.MarkMessageOffTopic(userMsgID, rec.Usage.OffTopic, rec.Usage.OffTopicReason); err != nil {
		logger.L().Warnw("mark off-topic failed", "session", sessionID, "message", userMsgID, "err", err)
	}
	if rec.Usage.OffTopic != nil {
		logger.L().Infow("off-topic judged",
			"session", sessionID, "offTopic", *rec.Usage.OffTopic, "reason", rec.Usage.OffTopicReason)
	}
	if asm.hasPlayer || asm.hasAsset {
		if err := s.repo.MarkSessionContext(sessionID, asm.hasPlayer, asm.hasAsset); err != nil {
			logger.L().Warnw("mark session context failed", "session", sessionID, "err", err)
		}
	}
	if err := s.repo.Touch(sessionID); err != nil {
		logger.L().Errorw("touch session failed", "session", sessionID, "err", err)
	}
}

// buildHistory 组装发给 AI 服务的历史消息。
// 双重限制：条数上限（由查询承担）+ token 预算（此处裁剪），从最近一条往前累计。
// 规则：
//   - 丢弃尾部与本次提问相同的那条 user 消息——提问已由 prompt 字段单独发送，重复下发会重复计费并干扰模型
//   - 过滤降级兜底文案：它不承载有效信息，留在上下文里会污染后续对话
//   - 超预算即截断，但**始终保留最新一条**（否则模型会丢失最近的语境）
func buildHistory(recent []model.ChatMessage, question string) []Message {
	picked := make([]Message, 0, len(recent))
	used := 0

	for i := len(recent) - 1; i >= 0; i-- {
		msg := recent[i]
		if i == len(recent)-1 && msg.Role == "user" && strings.TrimSpace(msg.Content) == question {
			continue
		}
		if msg.Role == "assistant" && msg.Content == aiDegradeReply {
			continue
		}

		cost := roughTokens(msg.Content)
		if len(picked) > 0 && used+cost > historyTokenBudget {
			break
		}
		used += cost
		picked = append(picked, Message{Role: msg.Role, Content: msg.Content})
	}

	// 反转为时间正序
	for i, j := 0, len(picked)-1; i < j; i, j = i+1, j-1 {
		picked[i], picked[j] = picked[j], picked[i]
	}
	return picked
}

// buildContext 组装发送给 AI 服务的业务上下文，两条链路任一有内容即推送：
//   - V2 资产资料（公开数据，任何用户都生效）：命中角色名时给出库里的权威推荐词条
//   - V1 玩家档案（私有数据，需登录且已绑定 UID）：分级注入展柜角色面板
// 任何失败都不阻塞对话（缺上下文只是回答不够精准）。
func (s *ChatService) buildContext(ctx context.Context, userID uint, question string) contextAssembly {
	var summary string
	if s.players != nil {
		// ⚠️ 三块资料（【角色机制】→【角色攻略】→【游戏资料】）统一在 SummaryForQuestion 里组装 ✓：
		// 它是**唯一出口** —— 离线评测器复刻同一条链（cmd/mechanicsimport -questions/-out ✓）
		// ⇒ 线上与评测拿到的是**逐字相同**的注入文本 ✓（2026-10-13 接入机制卡时收敛的 ✓）
		var stats SummaryStats
		summary, stats = s.players.SummaryForQuestion(question)
		if stats.MechanicCards > 0 || len(stats.Rules) > 0 {
			logger.L().Infow("ai character mechanic attached",
				"cards", stats.MechanicCards, "rules", strings.Join(stats.Rules, ","))
		}
		if stats.GuideCards > 0 {
			logger.L().Infow("ai character guide attached", "cards", stats.GuideCards)
		}
		if stats.Refs > 0 || stats.Sets > 0 {
			logger.L().Infow("ai asset reference attached", "matched", stats.Refs, "sets", stats.Sets)
		}
	}

	rc := s.buildPlayerContext(ctx, userID, question)
	asm := contextAssembly{rc: rc, hasPlayer: rc != nil, hasAsset: summary != ""}

	switch {
	case rc == nil && summary == "":
		return asm
	case rc == nil:
		// 匿名用户也能拿到公开资料：数值类问题不再靠模型记忆
		asm.rc = &RequestContext{Summary: summary}
		return asm
	}

	if summary != "" {
		// summary 渲染时置于玩家档案之前：先给「权威资料」，再给「你的面板」
		if rc.Summary == "" {
			rc.Summary = summary
		} else {
			rc.Summary = summary + "\n" + rc.Summary
		}
	}
	return asm
}

// buildPlayerContext 玩家档案上下文（V1）：仅对「已登录且已绑定 UID」的用户生效
// 任何失败都不阻塞对话（缺上下文只是回答不够个性化）
//
// ⚠️ 2026-10-12 修（用户报"聊天页取不到绑定 UID 的角色信息"✓）：此前这里有**三处静默放弃** ✗ ——
//  ① 没绑定 ⇒ 直接 return nil（**不留任何日志** ✗）；② 上游查询失败 ⇒ 只 warn、角色信息整个丢 ✗；
//  ③ 展柜为空 ⇒ BuildPlayerContext 返回 nil ✗。三者对用户是**同一个结果**（派蒙看不到角色）✗，
//  却都不说为什么 ✓ ⇒ 现在：**能兜底就兜底**（用「已收录角色池」✓）、**放弃就留痕**（Info/Warn ✓）。
func (s *ChatService) buildPlayerContext(ctx context.Context, userID uint, question string) *RequestContext {
	if userID == 0 || s.players == nil {
		return nil
	}

	uid, _, err := s.players.Binding(userID)
	if err != nil {
		// ③ 留痕：读绑定失败（此前完全无痕 ✗）
		logger.L().Warnw("ai context skipped: read binding failed", "user", userID, "err", err)
		return nil
	}
	if uid == "" {
		// ③ 留痕：未绑定。这不是错误（用户可能刚解绑 ✓），但**必须留下痕迹**，
		// 否则"聊天没有角色信息"这件事在日志里查不到原因 ✗（今晚排查就卡在这 ✓）。
		logger.L().Infow("ai context skipped: no bound uid", "user", userID)
		return nil
	}

	profile, err := s.players.Query(ctx, uid, false) // 优先命中快照缓存
	if err != nil {
		// ① 兜底：上游挂了也应该认得"你的角色"✓（池子里是最近一次成功查询见过的 ✓）
		logger.L().Warnw("ai context: query profile failed, fall back to collected pool",
			"user", userID, "uid", uid, "err", err)
		profile = s.collectedProfile(uid)
	}
	// ① 展柜为空（私有 / 没摆）时同样兜底 —— 此前这条也直接放弃 ✗
	if profile == nil || len(profile.Characters) == 0 {
		if p := s.collectedProfile(uid); p != nil {
			logger.L().Infow("ai context: showcase empty, fall back to collected pool",
				"user", userID, "uid", uid, "collected", len(p.Characters))
			profile = p
		}
	}
	if profile == nil || len(profile.Characters) == 0 {
		// ③ 留痕：查到了、但一个角色都没有，且池子也是空的
		logger.L().Infow("ai context skipped: no showcase and empty collected pool", "user", userID, "uid", uid)
		return nil
	}

	rc := BuildPlayerContext(profile, question)
	if rc == nil {
		logger.L().Infow("ai context skipped: builder returned nil", "user", userID, "uid", uid)
		return nil
	}
	logger.L().Infow("ai context attached", "user", userID, "uid", uid, "goal", rc.Goal, "source", profile.Source)
	return rc
}

// collectedProfile 用「已收录角色池」拼一份**最小档案**（兜底专用 ✓，2026-10-12）。
//
// 池子 = 该 UID **历次成功查询**见过的角色（只增不删 ✓，换出展柜也不丢 ✓）⇒
// 上游抽风 / 展柜为空时，用它比"什么都给不出来"强 ✓。
// ⚠️ 它**不是实时面板** ⇒ Source 标成 "collected" ✓，由 BuildPlayerContext 在上下文里写明
// 「数据来自历史收录」✓ —— 否则模型会把历史面板当当前面板讲 ✗。
func (s *ChatService) collectedProfile(uid string) *dto.PlayerProfile {
	col, err := s.players.Characters(uid)
	if err != nil || col == nil || len(col.Characters) == 0 {
		return nil
	}
	cards := make([]dto.CharacterCard, 0, len(col.Characters))
	lastSeen := ""
	for _, c := range col.Characters {
		cards = append(cards, c.CharacterCard)
		if d := c.LastSeenAt.Format("2006-01-02"); d > lastSeen {
			lastSeen = d
		}
	}
	return &dto.PlayerProfile{
		Source:      "collected",
		UID:         uid,
		Characters:  cards,
		CollectedAt: lastSeen, // 供上下文标注"截至何时"✓
	}
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func titleOf(q string) string {
	r := []rune(q)
	if len(r) > 20 {
		return string(r[:20]) + "…"
	}
	return q
}
