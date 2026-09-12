package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

// 用 mock provider 跑通「Handler → Service → AIClient → SSE 帧」整条链路
// （不依赖数据库与真实模型：无 DB 时 repository 走降级，mock 客户端一次性回调整段文本）
func newChatTestServer(t *testing.T, provider string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := service.NewChatService(config.AIConfig{Provider: provider}, nil)
	r := gin.New()
	r.POST("/api/chat", NewChatHandler(svc).Chat)
	return r
}

func doChat(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 帧顺序必须是 session → delta* → done，且 content-type 为 text/event-stream
func TestChatStreamFrames(t *testing.T) {
	w := doChat(t, newChatTestServer(t, "mock"), `{"message":"你好呀派蒙","stream":true}`)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d：%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type 不符：%q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: session\n") {
		t.Fatalf("缺少 session 帧：%s", body)
	}
	if !strings.Contains(body, "event: delta\n") {
		t.Fatalf("缺少 delta 帧：%s", body)
	}
	if !strings.Contains(body, "event: done\n") {
		t.Fatalf("缺少 done 帧：%s", body)
	}
	if strings.Contains(body, "event: error\n") {
		t.Fatalf("mock 链路不应出现 error 帧：%s", body)
	}
	if strings.Index(body, "event: session") > strings.Index(body, "event: delta") {
		t.Fatalf("session 帧必须先于 delta：%s", body)
	}
	if strings.Index(body, "event: delta") > strings.Index(body, "event: done") {
		t.Fatalf("done 帧必须最后：%s", body)
	}

	// done 帧必须带 sessionId / tokens / elapsedMs
	done := lastDataOf(t, body, "done")
	var payload dto.ChatStreamDone
	if err := json.Unmarshal([]byte(done), &payload); err != nil {
		t.Fatalf("done 帧解析失败：%v（%s）", err, done)
	}
	if payload.SessionID == "" {
		t.Fatal("done 帧缺少 sessionId")
	}

	// session 帧与 done 帧的会话 ID 必须一致
	session := lastDataOf(t, body, "session")
	var s dto.ChatStreamSession
	if err := json.Unmarshal([]byte(session), &s); err != nil {
		t.Fatalf("session 帧解析失败：%v（%s）", err, session)
	}
	if s.SessionID == "" || s.SessionID != payload.SessionID {
		t.Fatalf("会话 ID 不一致：session=%q done=%q", s.SessionID, payload.SessionID)
	}
}

// 非流式请求仍返回统一信封（不能因为新增流式分支而破坏旧契约）
func TestChatNonStreamKeepsEnvelope(t *testing.T) {
	w := doChat(t, newChatTestServer(t, "mock"), `{"message":"你好"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d：%s", w.Code, w.Body.String())
	}
	var env struct {
		Code int               `json:"code"`
		Data dto.ChatResponse  `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应解析失败：%v（%s）", err, w.Body.String())
	}
	if env.Code != 0 || env.Data.SessionID == "" || env.Data.Reply == "" {
		t.Fatalf("信封内容不符：%s", w.Body.String())
	}
}

// 参数校验失败仍返回 400 信封（流式与非流式共用同一个入口）
func TestChatRejectsEmptyMessage(t *testing.T) {
	w := doChat(t, newChatTestServer(t, "mock"), `{"message":""}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望 400，实际 %d：%s", w.Code, w.Body.String())
	}
}

// lastDataOf 取出指定事件的最后一个 data 载荷
func lastDataOf(t *testing.T, body, event string) string {
	t.Helper()
	chunks := strings.Split(body, "\n\n")
	for i := len(chunks) - 1; i >= 0; i-- {
		lines := strings.Split(strings.TrimSpace(chunks[i]), "\n")
		if len(lines) < 2 || lines[0] != "event: "+event {
			continue
		}
		return strings.TrimPrefix(lines[1], "data: ")
	}
	t.Fatalf("未找到 %s 帧：%s", event, body)
	return ""
}
