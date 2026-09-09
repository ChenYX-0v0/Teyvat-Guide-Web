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

// ChatService 对话业务编排
type ChatService struct {
	cfg  config.AIConfig
	ai   AIClient
	repo *repository.ChatRepository
}

func NewChatService(cfg config.AIConfig) *ChatService {
	return &ChatService{
		cfg:  cfg,
		ai:   NewAIClient(cfg),
		repo: repository.NewChatRepository(),
	}
}

// Ask 处理一次提问
func (s *ChatService) Ask(ctx context.Context, sessionID, question string) (string, string, int, int64, error) {
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

	reply, tokens, err := s.ai.Complete(ctx, s.cfg.System, ctxMsgs, question)
	if err != nil {
		return "", "", 0, 0, err
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

	if err := s.repo.Touch(sessionID); err != nil {
		logger.L().Errorw("touch session failed", "session", sessionID, "err", err)
	}

	return sessionID, reply, tokens, time.Since(start).Milliseconds(), nil
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
