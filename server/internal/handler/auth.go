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

// registerRequest 手机号注册请求（2026-09-27）：手机号 + 短信验证码 + 设置密码。
// 昵称可选（缺省 = 脱敏手机号）；邀请码字段留给后续功能 ✓。
type registerRequest struct {
	Phone       string `json:"phone" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"displayName,omitempty"`
	// InviteCode 邀请码（2026-09-28）：`auth.require_invite_code=true` 时必填 ✓
	// **不在 binding 里标 required** ✗ —— 开关关着时它就该可以不填，由服务层按开关判定 ✓
	InviteCode string `json:"inviteCode,omitempty"`
}

type sendSMSCodeRequest struct {
	Phone string `json:"phone" binding:"required"`
	// InviteCode 同 registerRequest：开关开着时**不填/不对都不发短信** ✓
	InviteCode string `json:"inviteCode,omitempty"`
}

// RegisterConfig GET /api/auth/config —— 注册入口可用性（**公开接口**）
//
// 前端据此决定显示"手机号注册"还是"邀请制"提示：`allowRegister=false`（当前线上口径）时
// 注册接口本身也会 403，所以前端不能硬编码入口 ✓。
func (h *AuthHandler) RegisterConfig(c *gin.Context) {
	response.OK(c, h.svc.RegisterOptions())
}

// SendSMS POST /api/auth/sms/send —— 发送注册验证码（**公开接口**）
//
// 防刷靠三重限速（同号 60s / 同号日上限 / 同 IP 小时上限，见 service/sms.go ✓）：
// 短信按条计费，被刷就是直接烧钱 ✗。响应体**不含验证码** ✗。
func (h *AuthHandler) SendSMS(c *gin.Context) {
	var req sendSMSCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	if err := h.svc.SendSMSCode(req.Phone, c.ClientIP(), req.InviteCode); err != nil {
		switch {
		// 邀请码：不填 / 不对 ⇒ 明确告知（**此时一条短信都没发** ✓，用户口径见 service/invite.go）
		case errors.Is(err, service.ErrInviteRequired),
			errors.Is(err, service.ErrInviteInvalid),
			errors.Is(err, service.ErrInviteUsed):
			response.BadRequest(c, err.Error())
		// 注册关闭时不发码（否则"不公开部署"也会被刷短信费 ✗）
		case errors.Is(err, service.ErrRegisterClosed):
			response.Fail(c, http.StatusForbidden, 403, err.Error())
		case errors.Is(err, service.ErrPhoneInvalid):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrSMSDisabled):
			response.Fail(c, http.StatusServiceUnavailable, 503, err.Error())
		default:
			var limited *service.ErrSMSRateLimited
			if errors.As(err, &limited) {
				response.FailWithData(c, http.StatusTooManyRequests, 429, limited.Error(), gin.H{
					"retryAfterSeconds": int(limited.RetryAfter.Seconds()) + 1, // 向上取整，别让前端倒计时差 1 秒就撞墙
				})
				return
			}
			logger.L().Warnw("send sms code failed", "phone", service.MaskPhone(req.Phone), "err", err)
			response.Fail(c, http.StatusServiceUnavailable, 503, err.Error())
		}
		return
	}
	opts := h.svc.RegisterOptions()
	response.OK(c, gin.H{
		"resendAfterSeconds": opts.SMSResendAfterSeconds,
		"expiresInSeconds":   opts.SMSCodeTTLSeconds,
	})
}

// Register POST /api/auth/register —— 手机号注册（需要前置发送验证码）
// 不公开 / 邀请制部署（`auth.allow_register=false`）下直接 403：账号由后台「新增用户」代建。
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	u, err := h.svc.Register(req.Phone, req.Code, req.Password, req.DisplayName, req.InviteCode)
	switch {
	case err == nil:
		logger.L().Infow("user registered", "username", u.Username, "phone", service.MaskPhone(u.Username))
		response.OK(c, gin.H{"id": u.ID, "username": u.Username, "displayName": u.DisplayName})
	case errors.Is(err, service.ErrRegisterClosed):
		response.Fail(c, http.StatusForbidden, 403, err.Error())
	case errors.Is(err, service.ErrPhoneTaken), errors.Is(err, service.ErrUsernameTaken):
		response.Fail(c, http.StatusConflict, 409, err.Error())
	case errors.Is(err, service.ErrUserInvalid),
		errors.Is(err, service.ErrPhoneInvalid),
		errors.Is(err, service.ErrSMSCodeWrong),
		errors.Is(err, service.ErrInviteRequired),
		errors.Is(err, service.ErrInviteInvalid),
		errors.Is(err, service.ErrInviteUsed):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrSMSDisabled):
		response.Fail(c, http.StatusServiceUnavailable, 503, err.Error())
	default:
		logger.L().Errorw("register failed", "phone", service.MaskPhone(req.Phone), "err", err)
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

// UpdateProfile PUT /api/auth/profile —— 改昵称（需 Bearer Token，2026-10-11）
//
// 只开放**昵称**：用户名（手机号）不可改 ✗ —— 它就是登录凭据 ✓；
// 游戏 UID 绑定走账号页的专用接口 ✓。空串语义 = 清空昵称（前端回落展示用户名 / 脱敏手机号 ✓），
// 所以**不设 `binding:"required"`** ✗（那会把"清空"判成参数错误 ✓）。
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	uid := c.GetUint("uid")
	if uid == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "未登录")
		return
	}
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	u, err := h.svc.UpdateDisplayName(uid, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Fail(c, http.StatusUnauthorized, 401, "账号不存在")
		case errors.Is(err, service.ErrUserInvalid):
			response.BadRequest(c, err.Error())
		default:
			logger.L().Errorw("update profile failed", "uid", uid, "err", err)
			response.Internal(c, "保存失败，请重试")
		}
		return
	}
	response.OK(c, gin.H{"displayName": u.DisplayName})
}

// ChangePassword POST /api/auth/password —— 改密码（需 Bearer Token，2026-10-11）
//
// ⚠️ 错误码刻意分两档：
//   - "当前密码不对" / "新密码太短" / "与旧密码相同" ⇒ **400**（表单问题 ✓）。
//     绝不能给 401 ✗ —— 前端把 401 当登录态失效，会直接把人踢回登录页 ✓，既误导又难用 ✗。
//   - 未登录 / 账号不存在 ⇒ 401 ✓。
//
// ⚠️ 已知边界（写进 docs/web/04 §4.2）：改密**不会吊销其他设备已签发的 Token** ✗ ——
// JWT 里只有 uid/username/exp，没有"密码版本"位。要做"改密即踢下线"需加列 + 中间件校验，
// 属独立一块，单独排期 ✓。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	uid := c.GetUint("uid")
	if uid == 0 {
		response.Fail(c, http.StatusUnauthorized, 401, "未登录")
		return
	}
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数有误："+err.Error())
		return
	}
	err := h.svc.ChangePassword(uid, req.OldPassword, req.NewPassword)
	switch {
	case err == nil:
		// ⚠️ 只记 uid，**绝不记密码** ✗（日志会进文件、也可能被采集 ✓）
		logger.L().Infow("password changed", "uid", uid)
		response.OK(c, gin.H{"changed": true})
	case errors.Is(err, service.ErrUserNotFound):
		response.Fail(c, http.StatusUnauthorized, 401, "账号不存在")
	case errors.Is(err, service.ErrOldPasswordWrong),
		errors.Is(err, service.ErrPasswordSame),
		errors.Is(err, service.ErrUserInvalid):
		response.BadRequest(c, err.Error())
	default:
		logger.L().Errorw("change password failed", "uid", uid, "err", err)
		response.Internal(c, "修改失败，请重试")
	}
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
