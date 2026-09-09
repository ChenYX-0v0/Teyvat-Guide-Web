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

// BadRequest 参数错误
func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, CodeBadReq, message)
}

// Internal 服务器内部错误
func Internal(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, CodeInternal, message)
}
