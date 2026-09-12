package handler

import (
	"encoding/json"
	"fmt"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

// ChatHandler 对话接口
type ChatHandler struct {
	svc *service.ChatService
}

func NewChatHandler(svc *service.ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

func (h *ChatHandler) Chat(c *gin.Context) {
	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}

	if req.Stream {
		h.chatStream(c, req)
		return
	}

	// 已登录用户（可选鉴权注入）会额外获得基于其绑定账号的个性化上下文
	sessionID, reply, tokens, elapsed, err := h.svc.Ask(
		c.Request.Context(), c.GetUint("uid"), req.SessionID, req.Message,
	)
	if err != nil {
		logger.L().Errorw("chat failed", "err", err)
		response.Internal(c, "服务开小差了："+err.Error())
		return
	}

	response.OK(c, dto.ChatResponse{
		SessionID: sessionID,
		Reply:     reply,
		Tokens:    tokens,
		ElapsedMs: elapsed,
	})
}

// chatStream 以 SSE 输出增量回复：session → delta* → (done | error)
// 契约见 docs/04-API接口文档.md §2.1。
func (h *ChatHandler) chatStream(c *gin.Context, req dto.ChatRequest) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 关闭反向代理缓冲，否则流式会被攒包一次性吐出

	w := &sseWriter{writer: c.Writer}
	sessionID, tokens, elapsed, err := h.svc.AskStream(
		c.Request.Context(), c.GetUint("uid"), req.SessionID, req.Message,
		func(id string) error { return w.send("session", dto.ChatStreamSession{SessionID: id}) },
		func(text string) error { return w.send("delta", dto.ChatStreamDelta{Text: text}) },
	)
	if err != nil {
		// 客户端已断开：连接不可写，静默结束（避免无意义的写日志）
		if c.Request.Context().Err() != nil {
			return
		}
		logger.L().Errorw("chat stream failed", "err", err, "session", sessionID)
		if sendErr := w.send("error", dto.ChatStreamError{Message: service.AIStreamInterruptedReply}); sendErr != nil {
			logger.L().Warnw("write sse error frame failed", "err", sendErr, "session", sessionID)
		}
		return
	}

	if sendErr := w.send("done", dto.ChatStreamDone{
		SessionID: sessionID,
		Tokens:    tokens,
		ElapsedMs: elapsed,
	}); sendErr != nil {
		logger.L().Warnw("write sse done frame failed", "err", sendErr, "session", sessionID)
	}
}

// sseWriter 把事件写成 SSE 帧并立即 flush（不 flush 会被缓冲到整个请求结束，流式就失去意义）
type sseWriter struct {
	writer gin.ResponseWriter
}

func (w *sseWriter) send(event string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sse payload: %w", err)
	}
	if _, err := fmt.Fprintf(w.writer, "event: %s\ndata: %s\n\n", event, raw); err != nil {
		return fmt.Errorf("write sse frame: %w", err)
	}
	w.writer.Flush()
	return nil
}
