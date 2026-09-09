package service

import (
	"context"
	"fmt"
	"strings"

	"teyvat-guide/server/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

// AIClient 大模型能力抽象，方便替换不同供应商
type AIClient interface {
	Complete(ctx context.Context, system string, history []Message, prompt string) (string, int, error)
}

// Message 对话上下文条目
type Message struct {
	Role    string
	Content string
}

// NewAIClient 依据配置创建实现
func NewAIClient(cfg config.AIConfig) AIClient {
	if cfg.Provider == "openai" && cfg.APIKey != "" {
		return &openAIClient{cfg: cfg}
	}
	return &mockClient{}
}

// ---------- Mock ----------

type mockClient struct{}

func (m *mockClient) Complete(_ context.Context, _ string, _ []Message, prompt string) (string, int, error) {
	reply := fmt.Sprintf(
		"[Mock] 已收到你的问题：%q。当前未配置真实大模型，请在 config/config.yaml 中设置 ai.provider=openai 与 api_key。",
		strings.TrimSpace(prompt),
	)
	return reply, len([]rune(reply)), nil
}

// ---------- OpenAI 兼容 ----------

type openAIClient struct {
	cfg config.AIConfig
}

func (o *openAIClient) Complete(ctx context.Context, system string, history []Message, prompt string) (string, int, error) {
	cfg := openai.DefaultConfig(o.cfg.APIKey)
	if o.cfg.BaseURL != "" {
		cfg.BaseURL = o.cfg.BaseURL
	}
	client := openai.NewClientWithConfig(cfg)

	msgs := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: system}}
	for _, h := range history {
		switch h.Role {
		case "user":
			msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: h.Content})
		case "assistant":
			msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: h.Content})
		}
	}
	msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: prompt})

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    o.cfg.Model,
		Messages: msgs,
	})
	if err != nil {
		return "", 0, err
	}
	if len(resp.Choices) == 0 {
		return "", 0, fmt.Errorf("empty completion")
	}
	return resp.Choices[0].Message.Content, resp.Usage.TotalTokens, nil
}
