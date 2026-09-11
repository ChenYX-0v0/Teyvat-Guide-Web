package middleware

// InternalKeyAuth 内网接口鉴权：请求头 X-Internal-Key 必须与 auth.internal_key 一致
// 该密钥独立于 用户 JWT / X-Admin-Key / X-AI-Key，禁止复用（安全基线见 04-用户体系与鉴权边界设计 §7）
// 未配置 internal_key 时路由不注册（见 router），此处再兜底拒绝，避免误开放

import (
	"crypto/subtle"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

func InternalKeyAuth(cfg config.AuthConfig) gin.HandlerFunc {
	expect := cfg.InternalKey
	return func(c *gin.Context) {
		key := c.GetHeader("X-Internal-Key")
		if expect == "" || key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(expect)) != 1 {
			response.Fail(c, 401, 401, "缺少或错误的 X-Internal-Key")
			c.Abort()
			return
		}
		c.Next()
	}
}
