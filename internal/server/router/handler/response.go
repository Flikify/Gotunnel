package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Response 统一 API 响应结构
type Response struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// 业务错误码定义
const (
	CodeSuccess       = 0
	CodeBadRequest    = 400
	CodeUnauthorized  = 401
	CodeForbidden     = 403
	CodeNotFound      = 404
	CodeConflict      = 409
	CodeInternalError = 500
	CodeBadGateway    = 502

	CodeClientNotOnline  = 1001
	CodePluginNotFound   = 1002
	CodeInvalidClientID  = 1003
	CodePluginDisabled   = 1004
	CodeConfigSyncFailed = 1005
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

// BadRequest 400 错误
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

// Unauthorized 401 错误
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
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

// BindJSON 绑定 JSON 并自动处理验证错误
func BindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		handleValidationError(c, err)
		return false
	}
	return true
}

// BindQuery 绑定查询参数并自动处理验证错误
func BindQuery(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		handleValidationError(c, err)
		return false
	}
	return true
}

// handleValidationError 处理验证错误
func handleValidationError(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		errs := make([]map[string]string, len(ve))
		for i, fe := range ve {
			errs[i] = map[string]string{
				"field":   fe.Field(),
				"message": getValidationMessage(fe),
			}
		}
		c.JSON(http.StatusBadRequest, Response{
			Code:    CodeBadRequest,
			Message: "validation failed",
			Data:    errs,
		})
		return
	}
	BadRequest(c, err.Error())
}

func getValidationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "this field is required"
	case "min":
		return "value is too short or too small"
	case "max":
		return "value is too long or too large"
	case "url":
		return "invalid URL format"
	case "oneof":
		return "value must be one of: " + fe.Param()
	default:
		return "validation failed on " + fe.Tag()
	}
}
