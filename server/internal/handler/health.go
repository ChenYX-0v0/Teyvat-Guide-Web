package handler

import (
	"time"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

const version = "0.1.0"

// HealthHandler 健康检查
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

func (h *HealthHandler) Health(c *gin.Context) {
	response.OK(c, dto.HealthResponse{
		Status:  "up",
		Version: version,
		Time:    time.Now().Format(time.RFC3339),
	})
}
