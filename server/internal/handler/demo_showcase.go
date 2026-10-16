package handler

import (
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

// DemoHandler 游客演示模式（**无需登录**）。
//
// 演示页只读这条接口：**不连 AI 服务、不查 Enka、不消耗 token、不落库会话** ✓
// （匿名已被禁用 AI 对话，见 shared-docs/05 §1；演示模式是它的替代体验）
type DemoHandler struct {
	svc *service.DemoShowcaseService
}

func NewDemoHandler(svc *service.DemoShowcaseService) *DemoHandler {
	return &DemoHandler{svc: svc}
}

// Showcase GET /api/demo/showcase
// 一次性返回：演示账号头 + 4 位演示角色（含预置的「AI 分析」）
func (h *DemoHandler) Showcase(c *gin.Context) {
	data, err := h.svc.Showcase()
	if err != nil {
		logger.L().Errorw("demo showcase failed", "err", err)
		response.Internal(c, "获取演示数据失败")
		return
	}
	response.OK(c, data)
}
