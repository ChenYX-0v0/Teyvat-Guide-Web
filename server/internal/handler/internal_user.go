package handler

// 内网用户管理接口（供后台运维系统调用）
// 路由挂 middleware.InternalKeyAuth，仅内网可达；契约见 04-用户体系与鉴权边界设计 §4

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

type InternalUserHandler struct {
	svc *service.UserAdminService
}

func NewInternalUserHandler(svc *service.UserAdminService) *InternalUserHandler {
	return &InternalUserHandler{svc: svc}
}

// ListUsers GET /internal/v1/admin/users
//
//	?keyword=&usageDate=2026-09-14&minDailyTokens=50000&onlyOffTopic=true&offTopicMin=1&page=1&pageSize=20
func (h *InternalUserHandler) ListUsers(c *gin.Context) {
	q := dto.AdminUserQuery{
		Keyword:      c.Query("keyword"),
		UsageDate:    c.Query("usageDate"),
		OnlyOffTopic: c.Query("onlyOffTopic") == "true",
		Page:         parseIntDefault(c.Query("page"), 1),
		PageSize:     parseIntDefault(c.Query("pageSize"), 20),
	}
	// minDailyTokens / offTopicMin 都允许为 0（= 不过滤），因此不能走 parseIntDefault（它把 0 当非法）
	if raw := c.Query("minDailyTokens"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			response.BadRequest(c, "minDailyTokens 必须为非负整数")
			return
		}
		q.MinDailyTokens = v
	}
	if raw := c.Query("offTopicMin"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			response.BadRequest(c, "offTopicMin 必须为非负整数")
			return
		}
		q.OffTopicMin = v
	}

	data, err := h.svc.List(q)
	if err != nil {
		logger.L().Errorw("list users failed", "err", err)
		response.Internal(c, "查询用户列表失败")
		return
	}
	response.OK(c, data)
}

// parseIntDefault 解析查询参数，非法或未提供时取默认值
func parseIntDefault(s string, def int) int {
	if v, err := parseInt(s); err == nil && v > 0 {
		return v
	}
	return def
}

// SetUserStatus POST /internal/v1/admin/users/status  { "id": 1, "banned": true }
func (h *InternalUserHandler) SetUserStatus(c *gin.Context) {
	var req dto.SetUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	err := h.svc.SetBanned(req.ID, req.Banned)
	switch {
	case err == nil:
		// 幂等：重复设置同一状态同样返回成功（data 为 null）
		response.OK[any](c, nil)
	case errors.Is(err, service.ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, 404, err.Error())
	default:
		logger.L().Errorw("set user status failed", "user", req.ID, "banned", req.Banned, "err", err)
		response.Internal(c, "更新用户状态失败")
	}
}

// CreateUser POST /internal/v1/admin/users  { "username": "...", "password": "...", "displayName": "..." }
// 后台代建账号——**邀请制（`auth.allow_register=false`）下的唯一开户入口**。
// 契约见 shared-docs/04-用户体系与鉴权边界设计.md §4.2。
func (h *InternalUserHandler) CreateUser(c *gin.Context) {
	var req dto.AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	data, err := h.svc.CreateUser(req)
	switch {
	case err == nil:
		response.OK(c, data)
	case errors.Is(err, service.ErrUsernameTaken):
		response.Fail(c, http.StatusConflict, 409, err.Error())
	case errors.Is(err, service.ErrUserInvalid):
		response.BadRequest(c, err.Error())
	default:
		logger.L().Errorw("create user failed", "username", req.Username, "err", err)
		response.Internal(c, "创建用户失败")
	}
}
