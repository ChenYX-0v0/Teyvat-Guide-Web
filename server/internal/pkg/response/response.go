package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope 统一响应结构
type Envelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

const (
	CodeOK       = 0
	CodeBadReq   = 400
	CodeUnauth   = 401
	CodeTooMany  = 429 // 配额 / 限流超限（契约见 docs/04-API接口文档.md §2.6）
	CodeInternal = 500
)

// OK 成功响应
func OK[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, Envelope[T]{Code: CodeOK, Message: "ok", Data: data})
}

// Fail 业务失败响应
func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Envelope[any]{Code: code, Message: message, Data: nil})
}

// FailWithData 业务失败响应（带 data）。
// 用于需要把结构一起下发的失败：如配额超限的 {used, limit, resetAt}（docs/04 §2.6）。
func FailWithData(c *gin.Context, httpStatus, code int, message string, data any) {
	c.JSON(httpStatus, Envelope[any]{Code: code, Message: message, Data: data})
}

// BadRequest 参数错误
func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, CodeBadReq, message)
}

// Internal 服务器内部错误
func Internal(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, CodeInternal, message)
}
