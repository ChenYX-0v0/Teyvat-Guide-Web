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
func (h *AuthHandler) Register(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	u, err := h.svc.Register(req.Username, req.Password, req.DisplayName)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			response.Fail(c, http.StatusConflict, 409, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	logger.L().Infow("user registered", "username", u.Username)
	response.OK(c, gin.H{"id": u.ID, "username": u.Username})
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
		logger.L().Warnw("login failed", "username", req.Username, "err", err)
		response.Internal(c, "登录失败")
		return
	}
	response.OK(c, gin.H{"token": token, "displayName": displayName})
}

// Me GET /api/auth/me —— 当前登录用户（需 Bearer Token）
func (h *AuthHandler) Me(c *gin.Context) {
	uid := c.GetUint("uid")
	username := c.GetString("username")
	if uid == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "未登录")
		return
	}
	logger.L().Debugw("me", "uid", uid, "username", username)
	response.OK(c, gin.H{"id": uid, "username": username})
}
