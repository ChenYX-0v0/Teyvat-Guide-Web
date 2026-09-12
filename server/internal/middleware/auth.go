package middleware

import (
	"net/http"
	"strings"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// UserStatusChecker 用户封禁检查（由 service.UserAdminService 注入，避免中间件直接依赖业务实现）
// 返回 true 表示该用户已被封禁
type UserStatusChecker func(userID uint) bool

// JWTAuth 校验 Bearer Token（未配置密钥或测试环境可按需跳过）
// statusChecker 非空时同时校验封禁状态：封禁用户的既有 Token 立即失效
func JWTAuth(cfg config.AuthConfig, statusChecker UserStatusChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, 401, 401, "缺少身份凭证")
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			response.Fail(c, 401, 401, "身份凭证无效")
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if uidF, ok := claims["uid"].(float64); ok {
				c.Set("uid", uint(uidF))
			}
			if username, ok := claims["username"].(string); ok {
				c.Set("username", username)
			}
		}

		if rejectBanned(c, statusChecker) {
			return
		}
		c.Next()
	}
}

// JWTAuthOptional 可选鉴权：未带 Token 时放行（匿名访问）；带 Token 时必须是合法 Token
//
// ⚠️ 关键区别：**带了无效/过期 Token 一律 401，绝不静默降级为匿名**。
// 静默降级会让用户以为还登录着，实际会话归属（`chat_sessions.user_id`）、个性化面板、
// 封禁联动全部失效，后台也认不出这些数据是谁的（2026-09-12 修）。
// 封禁用户即使携带合法 Token 也会被拒绝（无法借匿名路径绕过封禁）。
func JWTAuthOptional(cfg config.AuthConfig, statusChecker UserStatusChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.Next()
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			response.Fail(c, 401, 401, "登录状态已失效，请重新登录")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.Fail(c, 401, 401, "身份凭证无效")
			c.Abort()
			return
		}
		uidF, ok := claims["uid"].(float64)
		if !ok || uidF <= 0 {
			response.Fail(c, 401, 401, "身份凭证无效")
			c.Abort()
			return
		}
		c.Set("uid", uint(uidF))
		if username, ok := claims["username"].(string); ok {
			c.Set("username", username)
		}

		if rejectBanned(c, statusChecker) {
			return
		}
		c.Next()
	}
}

// rejectBanned 已封禁用户直接拒绝（403），返回是否已中断请求
// 仅在 Token 校验通过并注入身份后调用，避免把无效 Token 误判为封禁
func rejectBanned(c *gin.Context, statusChecker UserStatusChecker) bool {
	if statusChecker == nil {
		return false
	}
	uid := c.GetUint("uid")
	if uid == 0 {
		return false
	}
	if statusChecker(uid) {
		response.Fail(c, http.StatusForbidden, 403, "账号已被封禁，请联系管理员")
		c.Abort()
		return true
	}
	return false
}
