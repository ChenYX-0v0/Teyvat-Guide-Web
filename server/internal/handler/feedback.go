package handler

// 功能反馈（2026-10-12）

import (
	"errors"
	"net/http"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

type FeedbackHandler struct {
	svc *service.FeedbackService
}

func NewFeedbackHandler(svc *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{svc: svc}
}

// Submit POST /api/feedback —— 提交功能反馈（需 Bearer Token，2026-10-12）
//
// 入参：`{category?, content, contact?}` —— **没有 userId** ✗：提交者只从 Token 取 ✓
// （否则任何人都能替别人提反馈 ✓）。
//
// 错误码：内容不合规（太短 / 太长 / 联系方式超长）⇒ **400** ✓；未登录 ⇒ 401 ✓。
// 与 `POST /auth/password` 同一条理由：**表单填错绝不能给 401** ✗ —— 前端把 401 当登录态失效，
// 会直接把人踢回登录页 ✓，那对一个错别字来说惩罚太重 ✓。
//
// ⚠️ 日志**只记 id / 类别 / 字数**，**不记反馈内容** ✗：内容是用户自由文本，
// 可能夹着账号、联系方式等隐私 ✓（与"改密码不记密码"同一口径 ✓）。
func (h *FeedbackHandler) Submit(c *gin.Context) {
	uid := c.GetUint("uid")
	if uid == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "未登录")
		return
	}

	var req struct {
		Category string `json:"category"`
		Content  string `json:"content" binding:"required"`
		Contact  string `json:"contact"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}

	f, err := h.svc.Submit(uid, req.Category, req.Content, req.Contact)
	switch {
	case err == nil:
		logger.L().Infow("feedback submitted",
			"uid", uid, "id", f.ID, "category", f.Category, "length", len([]rune(f.Content)))
		response.OK(c, gin.H{"id": f.ID, "createdAt": f.CreatedAt})
	case errors.Is(err, service.ErrUserNotFound):
		response.Fail(c, http.StatusUnauthorized, 401, "账号不存在")
	case errors.Is(err, service.ErrFeedbackInvalid):
		response.BadRequest(c, err.Error())
	default:
		logger.L().Errorw("feedback submit failed", "uid", uid, "err", err)
		response.Internal(c, "提交失败，请稍后重试")
	}
}
