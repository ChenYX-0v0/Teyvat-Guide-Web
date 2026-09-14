package handler

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

// PresetHandler 预设问答（**无需登录**）：匿名用户在对话页看到的内容
type PresetHandler struct {
	svc *service.PresetQAService
}

func NewPresetHandler(svc *service.PresetQAService) *PresetHandler {
	return &PresetHandler{svc: svc}
}

// List GET /api/preset-qa
// 不调模型、不消耗 token、不落库（契约见 docs/04-API接口文档.md §2.5）
func (h *PresetHandler) List(c *gin.Context) {
	data, err := h.svc.List()
	if err != nil {
		logger.L().Errorw("list preset qa failed", "err", err)
		response.Internal(c, "获取预设问答失败")
		return
	}
	response.OK(c, data)
}

// ---------- 后台维护（内网接口，X-Internal-Key；契约见 docs/04-API接口文档.md §7.3） ----------

// AdminList GET /internal/v1/admin/preset-qa
// 含已下架条目（下架用 enabled=0，不做物理删除）
func (h *PresetHandler) AdminList(c *gin.Context) {
	data, err := h.svc.ListAll()
	if err != nil {
		logger.L().Errorw("admin list preset qa failed", "err", err)
		response.Internal(c, "获取预设问答失败")
		return
	}
	response.OK(c, data)
}

// Create POST /internal/v1/admin/preset-qa
func (h *PresetHandler) Create(c *gin.Context) {
	var req dto.PresetQASaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	data, err := h.svc.Create(req)
	if err != nil {
		h.writeSaveError(c, err, "新建预设问答失败")
		return
	}
	response.OK(c, data)
}

// Update PUT /internal/v1/admin/preset-qa/:id
func (h *PresetHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "id 必须为正整数")
		return
	}
	var req dto.PresetQASaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	data, err := h.svc.Update(uint(id), req)
	if err != nil {
		h.writeSaveError(c, err, "更新预设问答失败")
		return
	}
	response.OK(c, data)
}

// writeSaveError 统一错误映射：参数错误 400 / 不存在 404 / 其余 500
func (h *PresetHandler) writeSaveError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrPresetInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrPresetNotFound):
		response.Fail(c, http.StatusNotFound, 404, err.Error())
	default:
		logger.L().Errorw("preset qa save failed", "err", err)
		response.Internal(c, fallback)
	}
}
