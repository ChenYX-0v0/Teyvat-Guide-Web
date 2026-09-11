package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"teyvat-guide/server/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

// AIClient 大模型能力抽象，方便替换不同供应商
type AIClient interface {
	Complete(ctx context.Context, system string, history []Message, prompt string, rc *RequestContext) (string, int, error)
}

// Message 对话上下文条目
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RequestContext 传给 AI 服务的可选业务上下文（如玩家档案摘要）
// 约定：仅在该次请求内存中存在，AI 服务不得落库；总量 ≤ 1500 tokens
type RequestContext struct {
	Goal    string         `json:"goal,omitempty"`    // 意图：general | character_build | team_build | exploration
	Player  map[string]any `json:"player,omitempty"`  // 玩家档案（展柜角色：概览 + 相关角色明细）
	Summary string         `json:"summary,omitempty"` // 自由文本补充，AI 渲染时置于档案之前
}

// NewAIClient 依据配置创建实现：mock / openai / remote
func NewAIClient(cfg config.AIConfig) AIClient {
	switch cfg.Provider {
	case "remote":
		return newRemoteAIClient(cfg)
	case "openai":
		if cfg.APIKey != "" {
			return &openAIClient{cfg: cfg}
		}
		return &mockClient{}
	default:
		return &mockClient{}
	}
}

// ---------- Remote（独立 AI 派蒙服务） ----------

// 与 AI 服务约定的响应信封
type aiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type aiCompleteData struct {
	Reply  string `json:"reply"`
	Model  string `json:"model"`
	Tokens struct {
		Input  int `json:"input"`
		Output int `json:"output"`
		Total  int `json:"total"`
		Cached int `json:"cached"`
	} `json:"tokens"`
	FinishReason string `json:"finishReason"`
	ElapsedMs    int    `json:"elapsedMs"`
}

type remoteAIClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func newRemoteAIClient(cfg config.AIConfig) AIClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 65 * time.Second
	}
	return &remoteAIClient{
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *remoteAIClient) Complete(
	ctx context.Context, system string, history []Message, prompt string, rc *RequestContext,
) (string, int, error) {
	options := map[string]any{}
	if c.model != "" {
		options["model"] = c.model
	}
	body := map[string]any{
		"system":  system,
		"history": history,
		"prompt":  prompt,
		"options": options,
	}
	if rc != nil {
		body["context"] = rc
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", 0, fmt.Errorf("marshal ai request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/v1/complete", bytes.NewReader(raw),
	)
	if err != nil {
		return "", 0, fmt.Errorf("build ai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AI-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("call ai service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // Close 无有意义的错误可处理

	var env aiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", 0, fmt.Errorf("decode ai response (status=%d): %w", resp.StatusCode, err)
	}
	if env.Code != 0 {
		return "", 0, fmt.Errorf("ai service error: code=%d message=%s", env.Code, env.Message)
	}

	var data aiCompleteData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", 0, fmt.Errorf("decode ai data: %w", err)
	}
	return data.Reply, data.Tokens.Total, nil
}

// ---------- Mock ----------

type mockClient struct{}

func (m *mockClient) Complete(_ context.Context, _ string, _ []Message, prompt string, _ *RequestContext) (string, int, error) {
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

func (o *openAIClient) Complete(ctx context.Context, system string, history []Message, prompt string, _ *RequestContext) (string, int, error) {
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
