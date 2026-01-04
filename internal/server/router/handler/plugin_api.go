package handler

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/pkg/protocol"
)

// PluginAPIHandler 插件 API 代理处理器
type PluginAPIHandler struct {
	app AppInterface
}

// NewPluginAPIHandler 创建插件 API 代理处理器
func NewPluginAPIHandler(app AppInterface) *PluginAPIHandler {
	return &PluginAPIHandler{app: app}
}

// ProxyRequest 代理请求到客户端插件
// @Summary 代理插件 API 请求
// @Description 将请求代理到客户端的 JS 插件处理
// @Tags 插件 API
// @Accept json
// @Produce json
// @Security Bearer
// @Param clientID path string true "客户端 ID"
// @Param pluginName path string true "插件名称"
// @Param route path string true "插件路由"
// @Success 200 {object} object
// @Failure 404 {object} Response
// @Failure 502 {object} Response
// @Router /api/client/{clientID}/plugin/{pluginName}/{route} [get]
func (h *PluginAPIHandler) ProxyRequest(c *gin.Context) {
	clientID := c.Param("clientID")
	pluginName := c.Param("pluginName")
	route := c.Param("route")

	// 确保路由以 / 开头
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}

	// 检查客户端是否在线
	online, _, _ := h.app.GetServer().GetClientStatus(clientID)
	if !online {
		ClientNotOnline(c)
		return
	}

	// 读取请求体
	var body string
	if c.Request.Body != nil {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		body = string(bodyBytes)
	}

	// 构建请求头
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// 构建 API 请求
	apiReq := protocol.PluginAPIRequest{
		PluginName: pluginName,
		Method:     c.Request.Method,
		Path:       route,
		Query:      c.Request.URL.RawQuery,
		Headers:    headers,
		Body:       body,
	}

	// 发送请求到客户端
	resp, err := h.app.GetServer().ProxyPluginAPIRequest(clientID, apiReq)
	if err != nil {
		BadGateway(c, "Plugin request failed: "+err.Error())
		return
	}

	// 检查错误
	if resp.Error != "" {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    502,
			"message": resp.Error,
		})
		return
	}

	// 设置响应头
	for key, value := range resp.Headers {
		c.Header(key, value)
	}

	// 返回响应
	c.String(resp.Status, resp.Body)
}

// ProxyPluginAPIRequest 接口方法声明 - 添加到 ServerInterface
type PluginAPIProxyInterface interface {
	ProxyPluginAPIRequest(clientID string, req protocol.PluginAPIRequest) (*protocol.PluginAPIResponse, error)
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type     string `json:"type"`      // none, basic, token
	Username string `json:"username"`  // Basic Auth 用户名
	Password string `json:"password"`  // Basic Auth 密码
	Token    string `json:"token"`     // Token 认证
}

// BasicAuthMiddleware 创建 Basic Auth 中间件
func BasicAuthMiddleware(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, pass, ok := c.Request.BasicAuth()
		if !ok || user != username || pass != password {
			c.Header("WWW-Authenticate", `Basic realm="Plugin"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Unauthorized",
			})
			return
		}
		c.Next()
	}
}

// WithTimeout 带超时的请求处理
func WithTimeout(timeout time.Duration, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置请求超时
		c.Request = c.Request.WithContext(c.Request.Context())
		handler(c)
	}
}
