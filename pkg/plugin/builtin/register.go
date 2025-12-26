package builtin

import "github.com/gotunnel/pkg/plugin"

// 全局插件注册表
var registry []plugin.ProxyHandler

// Register 插件自注册函数，由各插件的 init() 调用
func Register(handler plugin.ProxyHandler) {
	registry = append(registry, handler)
}

// GetAll 返回所有已注册的内置插件
func GetAll() []plugin.ProxyHandler {
	return registry
}
