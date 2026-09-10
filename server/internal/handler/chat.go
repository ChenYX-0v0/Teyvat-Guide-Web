package handler

import (
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
