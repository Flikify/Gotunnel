package handler

import (
	"crypto/subtle"

	"github.com/gin-gonic/gin"
	// removed router import
	"github.com/gotunnel/internal/server/http/dto"
	"github.com/gotunnel/pkg/auth"
	"github.com/gotunnel/pkg/security"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	username string
	password string
	jwtAuth  *auth.JWTAuth
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(username, password string, jwtAuth *auth.JWTAuth) *AuthHandler {
	return &AuthHandler{
		username: username,
		password: password,
		jwtAuth:  jwtAuth,
	}
}

// Login 用户登录
// @Summary 用户登录
// @Description 使用用户名密码登录，返回 JWT token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "登录信息"
// @Success 200 {object} Response{data=dto.LoginResponse}
// @Failure 400 {object} Response
// @Failure 401 {object} Response
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !BindJSON(c, &req) {
		return
	}

	// 验证用户名密码 (使用常量时间比较防止时序攻击)
	userMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.username)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.password)) == 1

	if !userMatch || !passMatch {
		security.LogWebLogin(c.ClientIP(), req.Username, false)
		Unauthorized(c, "invalid credentials")
		return
	}

	// 生成 token
	token, err := h.jwtAuth.GenerateToken(req.Username)
	if err != nil {
		InternalError(c, "failed to generate token")
		return
	}

	security.LogWebLogin(c.ClientIP(), req.Username, true)
	Success(c, dto.LoginResponse{Token: token})
}

// Check 检查 token 是否有效
// @Summary 检查 Token
// @Description 验证 JWT token 是否有效
// @Tags 认证
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.TokenCheckResponse}
// @Failure 401 {object} Response
// @Router /api/auth/check [get]
func (h *AuthHandler) Check(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		Unauthorized(c, "missing authorization header")
		return
	}

	const prefix = "Bearer "
	if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
		Unauthorized(c, "invalid authorization format")
		return
	}
	tokenStr := authHeader[len(prefix):]

	claims, err := h.jwtAuth.ValidateToken(tokenStr)
	if err != nil {
		Unauthorized(c, "invalid token")
		return
	}

	Success(c, dto.TokenCheckResponse{
		Valid:    true,
		Username: claims.Username,
	})
}
