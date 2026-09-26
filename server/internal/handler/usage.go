package handler

import (
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

// UsageHandler 埋点上报（**匿名可访问**，带 Token 时记 user_id）
type UsageHandler struct {
	svc *service.UsageService
}

func NewUsageHandler(svc *service.UsageService) *UsageHandler {
	return &UsageHandler{svc: svc}
}

// Ingest POST /api/events（契约见 shared-docs/07 §4.1）
//
// **任何情况都返回 200**：埋点不该让前端做任何事，也不该给探测者反馈——
// 未知事件、超限、参数不合法统一表现为 "accepted < 请求条数"，不返回具体原因。
func (h *UsageHandler) Ingest(c *gin.Context) {
	var req dto.UsageIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.OK(c, gin.H{"accepted": 0})
		return
	}

	// 可选登录：登录了才带 user_id；匿名一律 NULL（D1 不采集匿名标识）
	var userID *uint
	if v := c.GetUint("uid"); v > 0 {
		userID = &v
	}

	accepted := h.svc.Ingest(userID, c.ClientIP(), req)
	response.OK(c, gin.H{"accepted": accepted})
}
