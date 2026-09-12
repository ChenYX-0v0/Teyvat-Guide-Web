package handler

// 内网对话记录查看接口（只读，供后台运维系统调用）
// 路由挂 middleware.InternalKeyAuth，仅内网可达；契约见 admin/06-对话记录查看设计.md §4

import (
	"errors"
	"net/http"
	"strconv"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

type InternalChatHandler struct {
	svc *service.ChatAdminService
}

func NewInternalChatHandler(svc *service.ChatAdminService) *InternalChatHandler {
	return &InternalChatHandler{svc: svc}
}

// ListChatSessions GET /internal/v1/admin/chat/sessions?keyword=&userId=&uid=&onlyDegraded=&from=&to=&page=1&pageSize=20
func (h *InternalChatHandler) ListChatSessions(c *gin.Context) {
	q := dto.AdminChatQuery{
		Keyword:      c.Query("keyword"),
		UID:          c.Query("uid"),
		OnlyDegraded: c.Query("onlyDegraded") == "true",
		OnlyDeleted:  c.Query("onlyDeleted") == "true",
		From:         c.Query("from"),
		To:           c.Query("to"),
		Page:         parseIntDefault(c.Query("page"), 1),
		PageSize:     parseIntDefault(c.Query("pageSize"), 20),
	}
	if raw := c.Query("userId"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			response.BadRequest(c, "userId 必须为数字")
			return
		}
		q.UserID = uint(id)
	}

	data, err := h.svc.ListSessions(q)
	switch {
	case err == nil:
		response.OK(c, data)
	case errors.Is(err, service.ErrChatQueryInvalid):
		response.BadRequest(c, err.Error())
	default:
		logger.L().Errorw("list chat sessions failed", "err", err)
		response.Internal(c, "查询会话列表失败")
	}
}

// ListChatMessages GET /internal/v1/admin/chat/sessions/:sessionId/messages?afterId=&limit=
func (h *InternalChatHandler) ListChatMessages(c *gin.Context) {
	var afterID uint
	if raw := c.Query("afterId"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			response.BadRequest(c, "afterId 必须为数字")
			return
		}
		afterID = uint(v)
	}

	data, err := h.svc.Messages(c.Param("sessionId"), afterID, parseIntDefault(c.Query("limit"), 0))
	switch {
	case err == nil:
		response.OK(c, data)
	case errors.Is(err, service.ErrChatQueryInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrSessionNotFound):
		response.Fail(c, http.StatusNotFound, 404, err.Error())
	default:
		logger.L().Errorw("list chat messages failed", "err", err)
		response.Internal(c, "查询会话内容失败")
	}
}
