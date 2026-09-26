package handler

// 认证接口：注册 / 登录 / 当前用户

import (
	"errors"
	"net/http"

	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/pkg/response"
	"teyvat-guide/server/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type credentialsRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"displayName,omitempty"`
}

// Register POST /api/auth/register
// 不公开 / 邀请制部署（`auth.allow_register=false`）下直接 403：账号由后台「新增用户」代建。
func (h *AuthHandler) Register(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	u, err := h.svc.Register(req.Username, req.Password, req.DisplayName)
	switch {
	case err == nil:
		logger.L().Infow("user registered", "username", u.Username)
		response.OK(c, gin.H{"id": u.ID, "username": u.Username})
	case errors.Is(err, service.ErrRegisterClosed):
		response.Fail(c, http.StatusForbidden, 403, err.Error())
	case errors.Is(err, service.ErrUsernameTaken):
		response.Fail(c, http.StatusConflict, 409, err.Error())
	case errors.Is(err, service.ErrUserInvalid):
		response.BadRequest(c, err.Error())
	default:
		logger.L().Errorw("register failed", "username", req.Username, "err", err)
		response.Internal(c, "注册失败")
	}
}

// Login POST /api/auth/login —— 返回 JWT
func (h *AuthHandler) Login(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	token, displayName, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrBadCredentials) {
			response.Fail(c, http.StatusUnauthorized, 401, err.Error())
			return
		}
		if errors.Is(err, service.ErrUserBanned) {
			response.Fail(c, http.StatusForbidden, 403, err.Error())
			return
		}
		logger.L().Warnw("login failed", "username", req.Username, "err", err)
		response.Internal(c, "登录失败")
		return
	}
	response.OK(c, gin.H{"token": token, "displayName": displayName})
}

// Me GET /api/auth/me —— 当前登录用户 + 用户须知状态（需 Bearer Token）
//
// 前端据 `terms.currentVersion` 与 `terms.acceptedVersion` 是否一致决定要不要弹
// 「用户须知」强制阅读弹窗（版本号真源在主服务，见 service/terms.go）。
func (h *AuthHandler) Me(c *gin.Context) {
	uid := c.GetUint("uid")
	if uid == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "未登录")
		return
	}
	u, err := h.svc.Profile(uid)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Fail(c, http.StatusUnauthorized, 401, "账号不存在")
			return
		}
		logger.L().Errorw("me failed", "uid", uid, "err", err)
		response.Internal(c, "查询用户失败")
		return
	}
	terms, err := h.svc.TermsState(uid)
	if err != nil {
		logger.L().Warnw("terms state failed", "uid", uid, "err", err)
	}
	payload := gin.H{
		"id":          u.ID,
		"username":    u.Username,
		"displayName": u.DisplayName,
		"genshinUid":  u.GenshinUID,
	}
	if terms != nil {
		payload["terms"] = gin.H{
			"acceptedVersion": terms.AcceptedVersion,
			"acceptedAt":      terms.AcceptedAt,
			"currentVersion":  terms.CurrentVersion,
		}
	}
	response.OK(c, payload)
}

// AcceptTerms POST /api/auth/terms —— 记录用户已同意的用户须知版本（需 Bearer Token）
//
// 只接受服务端当前生效版本（`service.CurrentTermsVersion`）：旧页面/伪造请求会被 400 拒绝。
func (h *AuthHandler) AcceptTerms(c *gin.Context) {
	uid := c.GetUint("uid")
	if uid == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "未登录")
		return
	}
	var req struct {
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	if err := h.svc.AcceptTerms(uid, req.Version); err != nil {
		switch {
		case errors.Is(err, service.ErrTermsVersionInvalid):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			response.Fail(c, http.StatusUnauthorized, 401, "账号不存在")
		default:
			logger.L().Errorw("accept terms failed", "uid", uid, "err", err)
			response.Internal(c, "保存失败，请重试")
		}
		return
	}
	logger.L().Infow("terms accepted", "uid", uid, "version", service.CurrentTermsVersion)
	response.OK(c, gin.H{"version": service.CurrentTermsVersion})
}
