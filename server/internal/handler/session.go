package handler

// C 端会话管理接口（需登录，路由挂 middleware.JWTAuth）
// 契约见 docs/04-API接口文档.md §3

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

// ChatSessionHandler 会话列表 / 明细 / 重命名
type ChatSessionHandler struct {
	svc *service.ChatSessionService
}

func NewChatSessionHandler(svc *service.ChatSessionService) *ChatSessionHandler {
	return &ChatSessionHandler{svc: svc}
}

// List GET /api/sessions?page=&pageSize=
func (h *ChatSessionHandler) List(c *gin.Context) {
	data, err := h.svc.List(
		c.GetUint("uid"),
		parseIntDefault(c.Query("page"), 1),
		parseIntDefault(c.Query("pageSize"), 20),
	)
	if err != nil {
		h.fail(c, err, "查询会话列表失败")
		return
	}
	response.OK(c, data)
}

// Messages GET /api/sessions/:sessionId/messages?beforeId=&limit=
func (h *ChatSessionHandler) Messages(c *gin.Context) {
	var beforeID uint
	if raw := c.Query("beforeId"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			response.BadRequest(c, "beforeId 必须为数字")
			return
		}
		beforeID = uint(v)
	}

	data, err := h.svc.Messages(
		c.GetUint("uid"),
		c.Param("sessionId"),
		beforeID,
		parseIntDefault(c.Query("limit"), 0),
	)
	if err != nil {
		h.fail(c, err, "查询会话内容失败")
		return
	}
	response.OK(c, data)
}

// Rename PATCH /api/sessions/:sessionId  body { title }
func (h *ChatSessionHandler) Rename(c *gin.Context) {
	var req dto.RenameSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}

	data, err := h.svc.Rename(c.GetUint("uid"), c.Param("sessionId"), req.Title)
	if err != nil {
		h.fail(c, err, "重命名会话失败")
		return
	}
	response.OK(c, data)
}

// Delete DELETE /api/sessions/:sessionId
// 软删除：C 端不再可见（列表/明细/改名/续聊），数据保留供后台审计（见 docs/04 §2.4）。
func (h *ChatSessionHandler) Delete(c *gin.Context) {
	data, err := h.svc.Delete(c.GetUint("uid"), c.Param("sessionId"))
	if err != nil {
		h.fail(c, err, "删除会话失败")
		return
	}
	response.OK(c, data)
}

// fail 统一错误映射：参数错误 400 / 会话不存在（含非本人、已删除）404 / 其余 500
func (h *ChatSessionHandler) fail(c *gin.Context, err error, internalMsg string) {
	switch {
	case errors.Is(err, service.ErrChatQueryInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrSessionNotFound):
		// 不存在与不属于调用者返回同一语义，避免被用来探测会话是否存在
		response.Fail(c, http.StatusNotFound, http.StatusNotFound, "会话不存在")
	default:
		logger.L().Errorw(internalMsg, "err", err)
		response.Internal(c, internalMsg)
	}
}
