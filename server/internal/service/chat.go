package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"teyvat-guide/server/internal/config"
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
}

func NewChatService(cfg config.AIConfig, players *PlayerService) *ChatService {
	return &ChatService{
		cfg:     cfg,
		ai:      NewAIClient(cfg),
		repo:    repository.NewChatRepository(),
		players: players,
	}
}

// Ask 处理一次提问（非流式）
// userID 为 0 表示匿名（不推送玩家上下文）；非 0 且已绑定 UID 时注入玩家档案摘要
func (s *ChatService) Ask(ctx context.Context, userID uint, sessionID, question string) (string, string, int, int64, error) {
	start := time.Now()

	sessionID, history, rc, err := s.prepare(ctx, userID, sessionID, question)
	if err != nil {
		return "", "", 0, 0, err
	}

	reply, tokens, err := s.ai.Complete(ctx, s.cfg.System, history, question, rc)
	if err != nil {
		// 上游（AI 服务 / 模型）故障：只记日志，不向前端暴露内部细节，返回可读降级文案
		logger.L().Errorw("ai complete failed",
			"err", err,
			"session", sessionID,
			"provider", s.cfg.Provider,
		)
		reply, tokens = aiDegradeReply, 0
		// 降级回复同样落库（保持会话连续），标记便于事后排查上游可用性
		logger.L().Infow("chat degraded, fallback reply persisted", "session", sessionID)
	}

	s.persistAssistant(sessionID, reply, tokens)
	return sessionID, reply, tokens, time.Since(start).Milliseconds(), nil
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

	sessionID, history, rc, err := s.prepare(ctx, userID, sessionID, question)
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

	tokens, streamErr := s.ai.CompleteStream(ctx, s.cfg.System, history, question, rc, emit)
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
		case ctx.Err() != nil:
			// 客户端已断开：没必要再补写降级文案
		default:
			// 一个增量都没发出：按同步链路的降级策略给出可读文案，用户仍能看到回复
			tokens = 0
			streamErr = emit(aiDegradeReply)
			if streamErr == nil {
				logger.L().Infow("chat stream degraded, fallback reply persisted", "session", sessionID)
			}
		}
	}

	// 落库（含降级文案与中断时的部分内容），保证会话连续
	s.persistAssistant(sessionID, buf.String(), tokens)
	return sessionID, tokens, time.Since(start).Milliseconds(), streamErr
}

// prepare 提问前的公共编排：建会话 → 落库用户消息 → 组装历史与玩家上下文
func (s *ChatService) prepare(ctx context.Context, userID uint, sessionID, question string) (string, []Message, *RequestContext, error) {
	question = trim(question)
	if question == "" {
		return "", nil, nil, errors.New("问题不能为空")
	}

	if sessionID == "" {
		sessionID = uuid.NewString()
		if err := s.repo.CreateSession(&model.ChatSession{
			ID:        sessionID,
			Title:     titleOf(question),
			UserID:    userID, // 0 = 匿名会话
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			logger.L().Errorw("create session failed", "session", sessionID, "err", err)
		}
	}

	if err := s.repo.AppendMessage(&model.ChatMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   question,
		CreatedAt: time.Now(),
	}); err != nil {
		logger.L().Errorw("append user message failed", "session", sessionID, "err", err)
	}

	recent, err := s.repo.ListRecentMessages(sessionID, historyLimit)
	if err != nil {
		return "", nil, nil, err
	}

	return sessionID, buildHistory(recent, question), s.buildContext(ctx, userID, question), nil
}

// persistAssistant 落库助手回复并刷新会话活跃时间（失败只记日志，不影响本次回答）
func (s *ChatService) persistAssistant(sessionID, content string, tokens int) {
	if err := s.repo.AppendMessage(&model.ChatMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   content,
		Tokens:    tokens,
		CreatedAt: time.Now(),
	}); err != nil {
		logger.L().Errorw("append assistant message failed", "session", sessionID, "err", err)
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
func (s *ChatService) buildContext(ctx context.Context, userID uint, question string) *RequestContext {
	var refs []CharacterStatRef
	if s.players != nil {
		refs = s.players.StatWeightsForQuestion(question)
	}
	summary := BuildAssetSummary(refs)
	if summary != "" {
		logger.L().Infow("ai asset reference attached", "matched", len(refs))
	}

	rc := s.buildPlayerContext(ctx, userID, question)
	switch {
	case rc == nil && summary == "":
		return nil
	case rc == nil:
		// 匿名用户也能拿到公开资料：数值类问题不再靠模型记忆
		return &RequestContext{Summary: summary}
	}

	if summary != "" {
		// summary 渲染时置于玩家档案之前：先给「权威资料」，再给「你的面板」
		if rc.Summary == "" {
			rc.Summary = summary
		} else {
			rc.Summary = summary + "\n" + rc.Summary
		}
	}
	return rc
}

// buildPlayerContext 玩家档案上下文（V1）：仅对「已登录且已绑定 UID」的用户生效
// 任何失败都不阻塞对话（缺上下文只是回答不够个性化）
func (s *ChatService) buildPlayerContext(ctx context.Context, userID uint, question string) *RequestContext {
	if userID == 0 || s.players == nil {
		return nil
	}
	uid, _, err := s.players.Binding(userID)
	if err != nil || uid == "" {
		return nil
	}
	profile, err := s.players.Query(ctx, uid, false) // 优先命中快照缓存
	if err != nil {
		logger.L().Warnw("build ai context failed: query profile", "user", userID, "uid", uid, "err", err)
		return nil
	}
	rc := BuildPlayerContext(profile, question)
	if rc == nil {
		return nil
	}
	logger.L().Infow("ai context attached", "user", userID, "uid", uid, "goal", rc.Goal)
	return rc
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
