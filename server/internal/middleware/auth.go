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
			if uidF, ok := claims["uid"].(float64); ok {
				c.Set("uid", uint(uidF))
			}
			if username, ok := claims["username"].(string); ok {
				c.Set("username", username)
			}
		}
		c.Next()
	}
}

// JWTAuthOptional 可选鉴权：带合法 Token 时注入身份，未带 Token 时放行（匿名访问）
func JWTAuthOptional(cfg config.AuthConfig) gin.HandlerFunc {
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
		if err == nil && token.Valid {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if uidF, ok := claims["uid"].(float64); ok {
					c.Set("uid", uint(uidF))
				}
				if username, ok := claims["username"].(string); ok {
					c.Set("username", username)
				}
			}
		}
		c.Next()
	}
}
