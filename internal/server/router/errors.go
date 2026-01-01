package router

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ValidationError 验证错误详情
type ValidationError struct {
	Field   string `json:"field"`   // 字段名
	Message string `json:"message"` // 错误消息
}

// HandleValidationError 处理验证错误并返回统一格式
func HandleValidationError(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		errs := make([]ValidationError, len(ve))
		for i, fe := range ve {
			errs[i] = ValidationError{
				Field:   fe.Field(),
				Message: getValidationMessage(fe),
			}
		}
		c.JSON(http.StatusBadRequest, Response{
			Code:    CodeBadRequest,
			Message: "validation failed",
			Data:    errs,
		})
		return
	}

	// 非验证错误，返回通用错误消息
	BadRequest(c, err.Error())
}

// getValidationMessage 根据验证标签返回友好的错误消息
func getValidationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "this field is required"
	case "min":
		return "value is too short or too small"
	case "max":
		return "value is too long or too large"
	case "email":
		return "invalid email format"
	case "url":
		return "invalid URL format"
	case "oneof":
		return "value must be one of: " + fe.Param()
	case "alphanum":
		return "must contain only letters and numbers"
	case "alphanumunicode":
		return "must contain only letters, numbers and unicode characters"
	case "ip":
		return "invalid IP address"
	case "hostname":
		return "invalid hostname"
	case "clientid":
		return "must be 1-64 alphanumeric characters, underscore or hyphen"
	case "gte":
		return "value must be greater than or equal to " + fe.Param()
	case "lte":
		return "value must be less than or equal to " + fe.Param()
	case "gt":
		return "value must be greater than " + fe.Param()
	case "lt":
		return "value must be less than " + fe.Param()
	default:
		return "validation failed on " + fe.Tag()
	}
}

// BindJSON 绑定 JSON 并自动处理验证错误
// 返回 true 表示绑定成功，false 表示已处理错误响应
func BindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		HandleValidationError(c, err)
		return false
	}
	return true
}

// BindQuery 绑定查询参数并自动处理验证错误
// 返回 true 表示绑定成功，false 表示已处理错误响应
func BindQuery(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		HandleValidationError(c, err)
		return false
	}
	return true
}

// BindURI 绑定 URI 参数并自动处理验证错误
// 返回 true 表示绑定成功，false 表示已处理错误响应
func BindURI(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindUri(obj); err != nil {
		HandleValidationError(c, err)
		return false
	}
	return true
}
