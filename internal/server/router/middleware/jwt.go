package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/pkg/auth"
)

// JWTAuth JWT 认证中间件
func JWTAuth(jwtAuth *auth.JWTAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// 支持从 query 参数获取 token (用于 SSE 等不支持自定义 header 的场景)
		if authHeader == "" {
			if token := c.Query("token"); token != "" {
				authHeader = "Bearer " + token
			}
		}

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "missing authorization header",
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid authorization format",
			})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwtAuth.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid or expired token",
			})
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("username", claims.Username)
		c.Set("claims", claims)
		c.Next()
	}
}
