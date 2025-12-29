package plugin

import (
	"log"
	"sync"

	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/plugin/builtin"
)

// Manager 服务端 plugin 管理器
type Manager struct {
	registry *plugin.Registry
	mu       sync.RWMutex
}

// NewManager 创建 plugin 管理器
func NewManager() (*Manager, error) {
	registry := plugin.NewRegistry()

	m := &Manager{
		registry: registry,
	}

	// 注册内置 plugins
	if err := m.registerBuiltins(); err != nil {
		return nil, err
	}

	return m, nil
}

// registerBuiltins 注册内置 plugins
func (m *Manager) registerBuiltins() error {
	// 注册服务端插件
	if err := m.registry.RegisterAll(builtin.GetAll()); err != nil {
		return err
	}
	// 注册客户端插件
	for _, h := range builtin.GetAllClientPlugins() {
		if err := m.registry.RegisterClientPlugin(h); err != nil {
			return err
		}
	}
	log.Printf("[Plugin] Registered %d server plugins, %d client plugins",
		len(builtin.GetAll()), len(builtin.GetAllClientPlugins()))
	return nil
}

// GetHandler 返回指定代理类型的 handler
func (m *Manager) GetHandler(proxyType string) (plugin.ProxyHandler, error) {
	return m.registry.Get(proxyType)
}

// ListPlugins 返回所有可用的 plugins
func (m *Manager) ListPlugins() []plugin.PluginInfo {
	return m.registry.List()
}

// GetRegistry 返回插件注册表
func (m *Manager) GetRegistry() *plugin.Registry {
	return m.registry
}
