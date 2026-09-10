package middleware

// AdminKey 管理接口鉴权：请求头 X-Admin-Key 必须与 auth.admin_key 一致
// 未配置 auth.admin_key 时回落到 auth.jwt_secret（兼容旧行为，不推荐）

import (
	"crypto/subtle"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

func AdminKey(cfg config.AuthConfig) gin.HandlerFunc {
	expect := cfg.AdminAPIKey()
	return func(c *gin.Context) {
		key := c.GetHeader("X-Admin-Key")
		if key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(expect)) != 1 {
			response.Fail(c, 401, 401, "缺少或错误的 X-Admin-Key")
			c.Abort()
			return
		}
		c.Next()
	}
}
