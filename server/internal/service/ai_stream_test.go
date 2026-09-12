package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"teyvat-guide/server/internal/config"
)

// 用假上游验证 SSE 帧解析：delta 逐段回调、done 取真实 token、X-AI-Key 正确注入
func TestRemoteAIClientCompleteStream(t *testing.T) {
	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-AI-Key")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("响应写入器不支持 flush")
			return
		}
		frames := []string{
			"event: delta\ndata: {\"text\":\"胡桃\"}\n\n",
			"event: delta\ndata: {\"text\":\"是火系主C\"}\n\n",
			// 未知事件必须被忽略（契约允许服务端新增事件类型）
			"event: heartbeat\ndata: {}\n\n",
			"event: done\ndata: {\"tokens\":{\"total\":645},\"finishReason\":\"stop\",\"model\":\"deepseek-flash\",\"elapsedMs\":1849}\n\n",
		}
		for _, f := range frames {
			fmt.Fprint(w, f)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	client := newRemoteAIClient(config.AIConfig{
		Provider:       "remote",
		BaseURL:        srv.URL,
		APIKey:         "test-key",
		TimeoutSeconds: 5,
	})

	var got strings.Builder
	tokens, err := client.CompleteStream(context.Background(), "", nil, "你好呀派蒙", nil, func(text string) error {
		got.WriteString(text)
		return nil
	})
	if err != nil {
		t.Fatalf("流式补全失败：%v", err)
	}
	if got.String() != "胡桃是火系主C" {
		t.Fatalf("增量文本拼接不符：%q", got.String())
	}
	if tokens != 645 {
		t.Fatalf("期望 tokens=645，实际 %d", tokens)
	}
	if gotPath != "/v1/complete/stream" {
		t.Fatalf("请求路径不符：%s", gotPath)
	}
	if gotKey != "test-key" {
		t.Fatalf("X-AI-Key 注入不符：%q", gotKey)
	}
}

// SSE 模式下没有外层信封，上游错误只能靠 error 事件表达
func TestRemoteAIClientCompleteStreamErrorFrame(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: delta\ndata: {\"text\":\"胡桃\"}\n\n")
		fmt.Fprint(w, "event: error\ndata: {\"message\":\"上游模型响应超时\"}\n\n")
	}))
	defer srv.Close()

	client := newRemoteAIClient(config.AIConfig{
		Provider:       "remote",
		BaseURL:        srv.URL,
		TimeoutSeconds: 5,
	})

	var got strings.Builder
	_, err := client.CompleteStream(context.Background(), "", nil, "你好", nil, func(text string) error {
		got.WriteString(text)
		return nil
	})
	if err == nil {
		t.Fatal("期望返回错误，实际为 nil")
	}
	if !strings.Contains(err.Error(), "上游模型响应超时") {
		t.Fatalf("错误信息未包含上游归一化文案：%v", err)
	}
	// 出错前的增量仍应已回调（服务层据此判断"是否已产出内容"）
	if got.String() != "胡桃" {
		t.Fatalf("错误前的增量应已回调，实际：%q", got.String())
	}
}

// onDelta 返回错误（如客户端断连）时应立即中止本次流
func TestRemoteAIClientCompleteStreamAbortsOnDeltaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: delta\ndata: {\"text\":\"胡桃\"}\n\n")
		fmt.Fprint(w, "event: done\ndata: {\"tokens\":{\"total\":1}}\n\n")
	}))
	defer srv.Close()

	client := newRemoteAIClient(config.AIConfig{
		Provider:       "remote",
		BaseURL:        srv.URL,
		TimeoutSeconds: 5,
	})

	boom := fmt.Errorf("客户端已断开")
	_, err := client.CompleteStream(context.Background(), "", nil, "你好", nil, func(string) error {
		return boom
	})
	if err == nil || !strings.Contains(err.Error(), boom.Error()) {
		t.Fatalf("期望把 onDelta 的错误原样返回，实际：%v", err)
	}
}
