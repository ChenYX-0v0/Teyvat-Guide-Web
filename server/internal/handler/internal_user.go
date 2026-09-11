package handler

// 内网用户管理接口（供后台运维系统调用）
// 路由挂 middleware.InternalKeyAuth，仅内网可达；契约见 04-用户体系与鉴权边界设计 §4

import (
	"errors"
	"net/http"

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

// ListUsers GET /internal/v1/admin/users?keyword=&page=1&pageSize=20
func (h *InternalUserHandler) ListUsers(c *gin.Context) {
	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("pageSize"), 20)

	data, err := h.svc.List(c.Query("keyword"), page, pageSize)
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
