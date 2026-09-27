package handler

// 内网：邀请码管理（2026-09-28）
//
// 与 preset-qa / ai-prompts 同一套路：**只走内网**（`X-Internal-Key`，未配置则整组不注册 ✓），
// 后台（demo-app）将来接一个页面即可 ✓；本轮先提供接口 + 命令行生成（见 deploy/out ✓）。

import (
	"errors"
	"net/http"
	"strconv"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

type InviteAdminHandler struct {
	svc *service.InviteService
}

func NewInviteAdminHandler(svc *service.InviteService) *InviteAdminHandler {
	return &InviteAdminHandler{svc: svc}
}

// List GET /internal/v1/admin/invite-codes?unused=true&limit=200
//
// 返回 `{items, usable}` —— `usable` 是**可用码总数**（后台一眼看还剩几张入场券 ✓）。
func (h *InviteAdminHandler) List(c *gin.Context) {
	unused := c.Query("unused") == "true" || c.Query("unused") == "1"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	items, err := h.svc.List(unused, limit)
	if err != nil {
		logger.L().Errorw("list invite codes failed", "err", err)
		response.Internal(c, "查询失败")
		return
	}
	usable, err := h.svc.CountUsable()
	if err != nil {
		logger.L().Warnw("count usable invite codes failed", "err", err)
	}
	response.OK(c, gin.H{"items": items, "usable": usable, "total": len(items)})
}

// Create POST /internal/v1/admin/invite-codes —— `{note?, count?}` → 生成一批（默认 1 张 ✓）
func (h *InviteAdminHandler) Create(c *gin.Context) {
	var req struct {
		Note  string `json:"note,omitempty"`
		Count int    `json:"count,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	if req.Count == 0 {
		req.Count = 1
	}
	items, err := h.svc.Generate(req.Note, req.Count)
	if err != nil {
		logger.L().Errorw("generate invite codes failed", "count", req.Count, "err", err)
		response.Internal(c, "生成失败")
		return
	}
	codes := make([]string, 0, len(items))
	for _, it := range items {
		codes = append(codes, it.Code)
	}
	logger.L().Infow("invite codes generated", "count", len(items), "note", req.Note)
	response.OK(c, gin.H{"items": items, "codes": codes})
}

// SetEnabled PUT /internal/v1/admin/invite-codes/:code —— `{enabled}`（停用 = 收回 ✓，不删行留痕 ✓）
func (h *InviteAdminHandler) SetEnabled(c *gin.Context) {
	code := c.Param("code")
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	if err := h.svc.SetEnabled(code, *req.Enabled); err != nil {
		if errors.Is(err, service.ErrInviteInvalid) {
			response.Fail(c, http.StatusNotFound, 404, "邀请码不存在")
			return
		}
		logger.L().Errorw("set invite code enabled failed", "err", err)
		response.Internal(c, "保存失败")
		return
	}
	logger.L().Infow("invite code enabled changed", "enabled", *req.Enabled)
	response.OK(c, gin.H{"code": code, "enabled": *req.Enabled})
}
