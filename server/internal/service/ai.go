package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"teyvat-guide/server/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

// 流式响应的关联常量
const (
	// streamHeaderTimeout 流式请求的响应头超时（连不上/首字节不返回就尽快失败）
	streamHeaderTimeout = 20 * time.Second
	// streamMaxFrameBytes 单个 SSE 帧的最大长度（中文内容的 JSON 转义后可能较长）
	streamMaxFrameBytes = 1024 * 1024
)

// AIClient 大模型能力抽象，方便替换不同供应商
type AIClient interface {
	Complete(ctx context.Context, system string, history []Message, prompt string, rc *RequestContext) (string, int, error)
	// CompleteStream 流式补全：每收到一段增量文本调用一次 onDelta（返回错误即中止本次流）。
	// 返回值为上游给出的真实 token 用量（上游未返回则为 0）。
	CompleteStream(ctx context.Context, system string, history []Message, prompt string, rc *RequestContext, onDelta func(text string) error) (int, error)
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
	timeout time.Duration   // 非流式整体超时；流式下作为"空闲超时"（见 withIdleTimeout）
	http    *http.Client    // 非流式
	stream  *http.Client    // 流式：不设整体 Timeout，否则长回答会被截断
}

func newRemoteAIClient(cfg config.AIConfig) AIClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 65 * time.Second
	}

	var transport http.RoundTripper = http.DefaultTransport
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		clone := base.Clone()
		clone.ResponseHeaderTimeout = streamHeaderTimeout
		transport = clone
	}

	return &remoteAIClient{
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
		stream:  &http.Client{Transport: transport},
	}
}

// payload 组装 /v1/complete 与 /v1/complete/stream 共用的请求体（契约见 shared-docs/01 §3）
func (c *remoteAIClient) payload(system string, history []Message, prompt string, rc *RequestContext) map[string]any {
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
	return body
}

func (c *remoteAIClient) Complete(
	ctx context.Context, system string, history []Message, prompt string, rc *RequestContext,
) (string, int, error) {
	raw, err := json.Marshal(c.payload(system, history, prompt, rc))
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

// CompleteStream 对接 AI 服务 `/v1/complete/stream`（SSE，帧格式见 shared-docs/01 §6）。
// SSE 模式下没有外层信封：上游错误只能通过 error 事件表达，因此必须逐帧解析、不能只看 HTTP 状态码。
func (c *remoteAIClient) CompleteStream(
	ctx context.Context, system string, history []Message, prompt string, rc *RequestContext, onDelta func(text string) error,
) (int, error) {
	raw, err := json.Marshal(c.payload(system, history, prompt, rc))
	if err != nil {
		return 0, fmt.Errorf("marshal ai request: %w", err)
	}

	// 空闲看门狗：流式响应整体可能远超 timeout（长回答），但任意一段静默超过 timeout 即视为卡死
	activity := &atomic.Int64{}
	activity.Store(time.Now().UnixNano())
	ctx, cancel := withIdleTimeout(ctx, c.timeout, activity)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/v1/complete/stream", bytes.NewReader(raw),
	)
	if err != nil {
		return 0, fmt.Errorf("build ai stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-AI-Key", c.apiKey)

	resp, err := c.stream.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call ai stream: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// 鉴权/入参类失败仍返回统一信封（HTTP 状态码 == code）
		var env aiEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&env); err != nil || env.Code == 0 {
			return 0, fmt.Errorf("ai stream failed: status=%d", resp.StatusCode)
		}
		return 0, fmt.Errorf("ai service error: code=%d message=%s", env.Code, env.Message)
	}

	tokens := 0
	err = parseSSE(&activityReader{reader: resp.Body, activity: activity}, func(event, data string) error {
		switch event {
		case "delta":
			var payload struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return fmt.Errorf("decode ai delta: %w", err)
			}
			if payload.Text == "" {
				return nil
			}
			return onDelta(payload.Text)
		case "done":
			var payload struct {
				Tokens struct {
					Total int `json:"total"`
				} `json:"tokens"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return fmt.Errorf("decode ai done: %w", err)
			}
			tokens = payload.Tokens.Total
			return nil
		case "error":
			var payload struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return fmt.Errorf("ai service stream error (undecodable payload)")
			}
			if payload.Message == "" {
				return fmt.Errorf("ai service stream error")
			}
			// 上游 message 已归一化（不含密钥/堆栈），仅用于服务端日志
			return fmt.Errorf("ai service stream error: %s", payload.Message)
		default:
			// 未知事件忽略：契约允许服务端新增事件类型
			return nil
		}
	})
	return tokens, err
}

// parseSSE 解析 SSE 帧（`event: x` / `data: y`，空行分隔），逐帧回调
func parseSSE(r io.Reader, onFrame func(event, data string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), streamMaxFrameBytes)

	var event, data string
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		switch {
		case line == "":
			if event == "" && data == "" {
				continue
			}
			if err := onFrame(event, data); err != nil {
				return err
			}
			event, data = "", ""
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ai stream: %w", err)
	}
	return nil
}

// withIdleTimeout 空闲超时：activity 在 idle 时长内没有更新则取消上下文。
// 流式响应不能用 http.Client.Timeout（它按整段计时，长回答会被中途掐断）。
func withIdleTimeout(parent context.Context, idle time.Duration, activity *atomic.Int64) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	if idle <= 0 {
		return ctx, cancel
	}

	interval := idle / 4
	if interval < time.Second {
		interval = time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Since(time.Unix(0, activity.Load())) >= idle {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}

// activityReader 记录最后一次读到数据的时刻，供 withIdleTimeout 判断空闲
type activityReader struct {
	reader   io.Reader
	activity *atomic.Int64
}

func (r *activityReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.activity.Store(time.Now().UnixNano())
	}
	return n, err
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

func (m *mockClient) CompleteStream(ctx context.Context, system string, history []Message, prompt string, rc *RequestContext, onDelta func(text string) error) (int, error) {
	reply, tokens, err := m.Complete(ctx, system, history, prompt, rc)
	if err != nil {
		return 0, err
	}
	return tokens, onDelta(reply)
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

// CompleteStream 直连模式暂未接流式协议：复用整段补全后一次性回调，
// 保证 AIClient 语义一致（调用方无需按 provider 分支）。
func (o *openAIClient) CompleteStream(ctx context.Context, system string, history []Message, prompt string, rc *RequestContext, onDelta func(text string) error) (int, error) {
	reply, tokens, err := o.Complete(ctx, system, history, prompt, rc)
	if err != nil {
		return 0, err
	}
	return tokens, onDelta(reply)
}
