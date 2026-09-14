package handler

import (
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
