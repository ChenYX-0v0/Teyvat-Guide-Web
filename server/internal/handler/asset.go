package handler

// 资产映射管理接口（JWT 保护，见 router 的 /api/admin 组）

import (
	"fmt"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"
	"teyvat-guide/server/internal/service/asset"

	"github.com/gin-gonic/gin"
)

type AssetHandler struct {
	assets  *asset.Service
	players *service.PlayerService
}

func NewAssetHandler(assets *asset.Service, players *service.PlayerService) *AssetHandler {
	return &AssetHandler{assets: assets, players: players}
}

// Import POST /api/admin/assets/import —— 批量 upsert 映射数据，成功后自动重载
func (h *AssetHandler) Import(c *gin.Context) {
	var payload asset.ImportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "请求体有误："+err.Error())
		return
	}
	if err := h.assets.Import(payload); err != nil {
		logger.L().Warnw("asset import failed", "err", err)
		response.Internal(c, "导入失败："+err.Error())
		return
	}
	response.OK(c, gin.H{
		"characters": len(payload.Characters),
		"weapons":    len(payload.Weapons),
		"relicSets":  len(payload.RelicSets),
		"textMap":    len(payload.TextMap),
		"propMap":    len(payload.PropMap),
	})
}

// Missing GET /api/admin/assets/missing?limit=100 —— 缺漏清单（按命中次数倒序）
func (h *AssetHandler) Missing(c *gin.Context) {
	limit := 0
	if v, err := parseInt(c.Query("limit")); err == nil {
		limit = v
	}
	list, err := h.assets.MissingList(limit)
	if err != nil {
		response.Internal(c, err.Error())
		return
	}
	response.OK(c, gin.H{"items": list, "total": len(list)})
}

// Reload POST /api/admin/assets/reload[?purgeSnapshots=true] —— 手动触发内存映射重载
// purgeSnapshots=true 时同时清空档案快照（改了词条名/分级等翻译相关数据后必须清，否则旧翻译仍会命中）
func (h *AssetHandler) Reload(c *gin.Context) {
	purge := c.Query("purgeSnapshots") == "true"
	if err := h.players.ReloadAssets(purge); err != nil {
		response.Internal(c, err.Error())
		return
	}
	response.OK(c, gin.H{"reloaded": true, "snapshotsPurged": purge})
}

// PurgeSnapshots POST /api/admin/assets/purge-snapshots —— 仅清空档案快照
func (h *AssetHandler) PurgeSnapshots(c *gin.Context) {
	if err := h.players.PurgeSnapshots(); err != nil {
		response.Internal(c, err.Error())
		return
	}
	response.OK(c, gin.H{"purged": true})
}

func parseInt(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
