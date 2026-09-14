package handler

// 内网 AI 提示词只读接口（供 AI 派蒙服务拉取，PROMPT_STORE_MODE=http 时使用）
// 路由挂 middleware.InternalKeyAuth，仅内网可达；契约见 shared-docs/03-主服务对接说明.md §5

import (
	"strings"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

// InternalAIHandler 提示词读取
type InternalAIHandler struct {
	svc *service.AIPromptService
}

func NewInternalAIHandler(svc *service.AIPromptService) *InternalAIHandler {
	return &InternalAIHandler{svc: svc}
}

// GetAIPrompts GET /internal/v1/ai/prompts?scenes=paimon_base,paimon_rules,paimon_fewshot
func (h *InternalAIHandler) GetAIPrompts(c *gin.Context) {
	var scenes []string
	if raw := strings.TrimSpace(c.Query("scenes")); raw != "" {
		scenes = strings.Split(raw, ",")
	}

	data, err := h.svc.Current(scenes)
	if err != nil {
		logger.L().Errorw("get ai prompts failed", "err", err)
		response.Internal(c, "查询提示词失败")
		return
	}
	response.OK(c, data)
}
