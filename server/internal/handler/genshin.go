package handler

import (
	"errors"
	"net/http"
	"regexp"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"
	"teyvat-guide/server/internal/service/enka"

	"github.com/gin-gonic/gin"
)

var uidPattern = regexp.MustCompile(`^\d{9}$`)

// PlayerHandler 玩家查询接口（Enka 数据源）
type PlayerHandler struct {
	svc *service.PlayerService
}

func NewPlayerHandler(svc *service.PlayerService) *PlayerHandler {
	return &PlayerHandler{svc: svc}
}

// Player GET /api/genshin/player?uid=197794747[&refresh=true]
// 默认优先返回本地快照；refresh=true 强制调 Enka 上游并更新快照
func (h *PlayerHandler) Player(c *gin.Context) {
	uid := c.Query("uid")
	if !uidPattern.MatchString(uid) {
		response.BadRequest(c, "UID 格式有误，应为 9 位数字")
		return
	}
	forceRefresh := c.Query("refresh") == "true"

	profile, err := h.svc.Query(c.Request.Context(), uid, forceRefresh)
	if err != nil {
		status, message := mapQueryError(err)
		if status != http.StatusNotFound {
			logger.L().Warnw("query player failed", "uid", uid, "err", err)
		}
		response.Fail(c, status, status, message)
		return
	}
	response.OK(c, profile)
}

// mapQueryError 将上游错误映射为 HTTP 状态码与友好文案
func mapQueryError(err error) (int, string) {
	switch {
	case errors.Is(err, enka.ErrPlayerNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, enka.ErrGameMaintenance):
		return http.StatusServiceUnavailable, err.Error()
	case errors.Is(err, enka.ErrUpstreamLimited):
		return http.StatusServiceUnavailable, err.Error()
	default:
		return http.StatusBadGateway, "查询失败：" + err.Error()
	}
}

// Snapshot GET /api/genshin/player-snapshot?uid=xxx —— 读取最近一次查询落库的档案快照
func (h *PlayerHandler) Snapshot(c *gin.Context) {
	uid := c.Query("uid")
	if !uidPattern.MatchString(uid) {
		response.BadRequest(c, "UID 格式有误，应为 9 位数字")
		return
	}
	snapshot, err := h.svc.Snapshot(uid)
	if err != nil {
		logger.L().Warnw("read player snapshot failed", "uid", uid, "err", err)
		response.Internal(c, "读取快照失败")
		return
	}
	if snapshot == nil {
		response.Fail(c, 404, 404, "该 UID 尚无查询记录，请先调用 /api/genshin/player")
		return
	}
	response.OK(c, snapshot)
}
