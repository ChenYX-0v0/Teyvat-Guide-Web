package middleware

// AdminKey 管理接口鉴权：请求头 X-Admin-Key 必须与配置的密钥一致
// 密钥复用 auth.jwt_secret（也可拆分为独立的 admin_key 配置）

import (
	"crypto/subtle"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

func AdminKey(cfg config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-Admin-Key")
		if key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(cfg.JWTSecret)) != 1 {
			response.Fail(c, 401, 401, "缺少或错误的 X-Admin-Key")
			c.Abort()
			return
		}
		c.Next()
	}
}
