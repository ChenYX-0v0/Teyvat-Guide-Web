package handler

// AI 提示词后台维护接口（内网，X-Internal-Key）：新增版本 / 启用回滚。
// 契约见 shared-docs/03-主服务对接说明.md §5.2；后台页面「AI 提瓦特向导 → 提示词管理」。
//
// 刻意**不提供"修改内容"接口**：改文案一律"新增一版"，老版本留作回滚（内容按版本不可变）。

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

// InternalAIPromptAdminHandler 提示词维护
type InternalAIPromptAdminHandler struct {
	svc *service.AIPromptAdminService
}

func NewInternalAIPromptAdminHandler(svc *service.AIPromptAdminService) *InternalAIPromptAdminHandler {
	return &InternalAIPromptAdminHandler{svc: svc}
}

// AdminList GET /internal/v1/admin/ai-prompts
// 全部版本（含未启用），并标出每个场景当前生效版本
func (h *InternalAIPromptAdminHandler) AdminList(c *gin.Context) {
	data, err := h.svc.ListAll()
	if err != nil {
		logger.L().Errorw("admin list ai prompts failed", "err", err)
		response.Internal(c, "获取提示词失败")
		return
	}
	response.OK(c, data)
}

// Create POST /internal/v1/admin/ai-prompts
// 新增一版（version 由服务端分配；缺省启用）
func (h *InternalAIPromptAdminHandler) Create(c *gin.Context) {
	var req dto.AIPromptCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	data, err := h.svc.Create(req)
	if err != nil {
		h.writePromptError(c, err, "新增提示词版本失败")
		return
	}
	response.OK(c, data)
}

// SetEnabled PUT /internal/v1/admin/ai-prompts/:id
// 只接受 enabled 字段：启用 / 停用（停用当前生效版本 = 回滚到上一启用版本）
func (h *InternalAIPromptAdminHandler) SetEnabled(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "id 必须为正整数")
		return
	}
	var req dto.AIPromptEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "请求参数有误：enabled 必填")
		return
	}
	data, err := h.svc.SetEnabled(id, *req.Enabled)
	if err != nil {
		h.writePromptError(c, err, "更新提示词状态失败")
		return
	}
	response.OK(c, data)
}

// writePromptError 统一错误映射：参数错误 400 / 不存在 404 / 其余 500
func (h *InternalAIPromptAdminHandler) writePromptError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrAIPromptInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrAIPromptNotFound):
		response.Fail(c, http.StatusNotFound, 404, err.Error())
	default:
		logger.L().Errorw("ai prompt admin failed", "err", err)
		response.Internal(c, fallback)
	}
}
