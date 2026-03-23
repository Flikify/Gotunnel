package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 静态资源扩展名
var staticExtensions = []string{
	".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
	".woff", ".woff2", ".ttf", ".eot", ".map", ".json", ".html",
}

// isStaticRequest 检查是否是静态资源请求
func isStaticRequest(path string) bool {
	// 检查 /assets/ 路径
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	// 检查文件扩展名
	for _, ext := range staticExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		method := c.Request.Method

		c.Next()

		// 跳过静态资源请求的日志
		if isStaticRequest(path) {
			return
		}

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()

		if query != "" {
			path = path + "?" + query
		}

		log.Printf("[API] %s %s %d %v %s",
			method, path, status, latency, clientIP)
	}
}
