package middleware

import (
	"time"

	"teyvat-guide/server/internal/pkg/logger"

	"github.com/gin-gonic/gin"
)

// AccessLog 访问日志
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.L().Infow("access",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"cost", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	}
}
