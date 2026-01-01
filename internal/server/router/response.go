package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一 API 响应结构
type Response struct {
	Code    int         `json:"code"`              // 业务状态码: 0=成功, 非0=错误
	Data    interface{} `json:"data,omitempty"`    // 响应数据
	Message string      `json:"message,omitempty"` // 提示信息
}

// 业务错误码定义
const (
	CodeSuccess       = 0   // 成功
	CodeBadRequest    = 400 // 请求参数错误
	CodeUnauthorized  = 401 // 未授权
	CodeForbidden     = 403 // 禁止访问
	CodeNotFound      = 404 // 资源不存在
	CodeConflict      = 409 // 资源冲突
	CodeInternalError = 500 // 服务器内部错误
	CodeBadGateway    = 502 // 网关错误

	// 业务错误码 (1000+)
	CodeClientNotOnline  = 1001 // 客户端不在线
	CodePluginNotFound   = 1002 // 插件不存在
	CodeInvalidClientID  = 1003 // 无效的客户端ID
	CodePluginDisabled   = 1004 // 插件已禁用
	CodeConfigSyncFailed = 1005 // 配置同步失败
)

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: CodeSuccess,
		Data: data,
	})
}

// SuccessWithMessage 成功响应带消息
func SuccessWithMessage(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Data:    data,
		Message: message,
	})
}

// Error 错误响应
func Error(c *gin.Context, httpCode int, bizCode int, message string) {
	c.JSON(httpCode, Response{
		Code:    bizCode,
		Message: message,
	})
}

// ErrorWithData 错误响应带数据
func ErrorWithData(c *gin.Context, httpCode int, bizCode int, message string, data interface{}) {
	c.JSON(httpCode, Response{
		Code:    bizCode,
		Message: message,
		Data:    data,
	})
}

// BadRequest 400 错误
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

// Unauthorized 401 错误
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

// Forbidden 403 错误
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, CodeForbidden, message)
}

// NotFound 404 错误
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, CodeNotFound, message)
}

// Conflict 409 错误
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, CodeConflict, message)
}

// InternalError 500 错误
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, CodeInternalError, message)
}

// BadGateway 502 错误
func BadGateway(c *gin.Context, message string) {
	Error(c, http.StatusBadGateway, CodeBadGateway, message)
}

// ClientNotOnline 客户端不在线错误
func ClientNotOnline(c *gin.Context) {
	Error(c, http.StatusBadRequest, CodeClientNotOnline, "client not online")
}

// PartialSuccess 部分成功响应
func PartialSuccess(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeConfigSyncFailed,
		Data:    data,
		Message: message,
	})
}
