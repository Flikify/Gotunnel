package builtin

import "github.com/gotunnel/pkg/plugin"

// 全局插件注册表
var (
	serverPlugins []plugin.ProxyHandler
	clientPlugins []plugin.ClientHandler
)

// Register 注册服务端插件
func Register(handler plugin.ProxyHandler) {
	serverPlugins = append(serverPlugins, handler)
}

// RegisterClientPlugin 注册客户端插件
func RegisterClientPlugin(handler plugin.ClientHandler) {
	clientPlugins = append(clientPlugins, handler)
}

// GetAll 返回所有服务端插件
func GetAll() []plugin.ProxyHandler {
	return serverPlugins
}

// GetAllClientPlugins 返回所有客户端插件
func GetAllClientPlugins() []plugin.ClientHandler {
	return clientPlugins
}
