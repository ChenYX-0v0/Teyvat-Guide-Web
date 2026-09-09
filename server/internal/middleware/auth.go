package middleware

import (
	"strings"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth 校验 Bearer Token（未配置密钥或测试环境可按需跳过）
func JWTAuth(cfg config.AuthConfig) gin.HandlerFunc {
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
			if uid, ok := claims["uid"]; ok {
				c.Set("uid", uid)
			}
		}
		c.Next()
	}
}
