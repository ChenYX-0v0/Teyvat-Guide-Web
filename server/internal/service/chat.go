package service

import (
	"context"
	"errors"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"

	"github.com/google/uuid"
)

const historyLimit = 20

// aiDegradeReply AI 服务不可用时的兜底回复（保持会话连续，不让请求失败）
const aiDegradeReply = "派蒙现在有点晕乎乎的，暂时联系不上她的脑子……旅行者稍后再问我一次好不好？"

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

// Ask 处理一次提问
// userID 为 0 表示匿名（不推送玩家上下文）；非 0 且已绑定 UID 时注入玩家档案摘要
func (s *ChatService) Ask(ctx context.Context, userID uint, sessionID, question string) (string, string, int, int64, error) {
	question = trim(question)
	if question == "" {
		return "", "", 0, 0, errors.New("问题不能为空")
	}

	start := time.Now()

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

	history, err := s.repo.ListMessages(sessionID, historyLimit)
	if err != nil {
		return "", "", 0, 0, err
	}

	ctxMsgs := make([]Message, 0, len(history))
	for _, m := range history {
		ctxMsgs = append(ctxMsgs, Message{Role: m.Role, Content: m.Content})
	}

	rc := s.buildContext(ctx, userID, question)

	reply, tokens, err := s.ai.Complete(ctx, s.cfg.System, ctxMsgs, question, rc)
	degraded := false
	if err != nil {
		// 上游（AI 服务 / 模型）故障：只记日志，不向前端暴露内部细节，返回可读降级文案
		logger.L().Errorw("ai complete failed",
			"err", err,
			"session", sessionID,
			"provider", s.cfg.Provider,
		)
		reply, tokens, degraded = aiDegradeReply, 0, true
	}

	if err := s.repo.AppendMessage(&model.ChatMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   reply,
		Tokens:    tokens,
		CreatedAt: time.Now(),
	}); err != nil {
		logger.L().Errorw("append assistant message failed", "session", sessionID, "err", err)
	}
	if degraded {
		// 降级回复同样落库（保持会话连续），标记便于事后排查上游可用性
		logger.L().Infow("chat degraded, fallback reply persisted", "session", sessionID)
	}

	if err := s.repo.Touch(sessionID); err != nil {
		logger.L().Errorw("touch session failed", "session", sessionID, "err", err)
	}

	return sessionID, reply, tokens, time.Since(start).Milliseconds(), nil
}

// buildContext 组装玩家上下文：仅对「已登录且已绑定 UID」的用户生效
// 任何失败都不阻塞对话（缺上下文只是回答不够个性化）
func (s *ChatService) buildContext(ctx context.Context, userID uint, question string) *RequestContext {
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
