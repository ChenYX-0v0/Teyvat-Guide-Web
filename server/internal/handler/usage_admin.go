package handler

import (
	"net/http"
	"strconv"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

// UsageAdminHandler 后台「使用统计」看板的内网接口（X-Internal-Key）
// 契约：shared-docs/07-前端埋点与使用统计.md §6
type UsageAdminHandler struct {
	svc *service.UsageService
}

func NewUsageAdminHandler(svc *service.UsageService) *UsageAdminHandler {
	return &UsageAdminHandler{svc: svc}
}

// Overview GET /internal/v1/admin/usage/overview?from=&to=
func (h *UsageAdminHandler) Overview(c *gin.Context) {
	data, err := h.svc.Overview(c.Query("from"), c.Query("to"))
	if err != nil {
		logger.L().Errorw("usage overview failed", "err", err)
		response.Internal(c, "读取使用统计概览失败")
		return
	}
	response.OK(c, data)
}

// Events GET /internal/v1/admin/usage/events?from=&to=&dim=&limit=
func (h *UsageAdminHandler) Events(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	data, err := h.svc.EventRanking(c.Query("from"), c.Query("to"), c.Query("dim"), limit)
	if err != nil {
		logger.L().Errorw("usage events failed", "err", err)
		response.Internal(c, "读取功能排行榜失败")
		return
	}
	response.OK(c, gin.H{"items": data})
}

// Series GET /internal/v1/admin/usage/series?days=30
func (h *UsageAdminHandler) Series(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	data, err := h.svc.Series(days)
	if err != nil {
		logger.L().Errorw("usage series failed", "err", err)
		response.Internal(c, "读取趋势失败")
		return
	}
	response.OK(c, gin.H{"items": data})
}

// Funnel GET /internal/v1/admin/usage/funnel?from=&to=
func (h *UsageAdminHandler) Funnel(c *gin.Context) {
	data, err := h.svc.Funnel(c.Query("from"), c.Query("to"))
	if err != nil {
		logger.L().Errorw("usage funnel failed", "err", err)
		response.Internal(c, "读取漏斗失败")
		return
	}
	response.OK(c, gin.H{"steps": data})
}

// Export GET /internal/v1/admin/usage/export?from=&to=&event=
func (h *UsageAdminHandler) Export(c *gin.Context) {
	body, err := h.svc.ExportCSV(c.Query("from"), c.Query("to"), c.Query("event"))
	if err != nil {
		logger.L().Errorw("usage export failed", "err", err)
		response.Internal(c, "导出失败")
		return
	}
	name := "usage-events.csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.String(http.StatusOK, body)
}
