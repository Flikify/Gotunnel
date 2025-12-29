package builtin

import "github.com/gotunnel/pkg/plugin"

var (
	serverPlugins []plugin.ServerPlugin
	clientPlugins []plugin.ClientPlugin
)

// RegisterServer 注册服务端插件
func RegisterServer(handler plugin.ServerPlugin) {
	serverPlugins = append(serverPlugins, handler)
}

// RegisterClient 注册客户端插件
func RegisterClient(handler plugin.ClientPlugin) {
	clientPlugins = append(clientPlugins, handler)
}

// GetServerPlugins 返回所有服务端插件
func GetServerPlugins() []plugin.ServerPlugin {
	return serverPlugins
}

// GetClientPlugins 返回所有客户端插件
func GetClientPlugins() []plugin.ClientPlugin {
	return clientPlugins
}
