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

	if err := m.registerBuiltins(); err != nil {
		return nil, err
	}

	return m, nil
}

// registerBuiltins 注册内置 plugins
func (m *Manager) registerBuiltins() error {
	if err := m.registry.RegisterAllServer(builtin.GetServerPlugins()); err != nil {
		return err
	}
	for _, h := range builtin.GetClientPlugins() {
		if err := m.registry.RegisterClient(h); err != nil {
			return err
		}
	}
	log.Printf("[Plugin] Registered %d server, %d client plugins",
		len(builtin.GetServerPlugins()), len(builtin.GetClientPlugins()))
	return nil
}

// GetServer 返回服务端插件
func (m *Manager) GetServer(name string) (plugin.ServerPlugin, error) {
	return m.registry.GetServer(name)
}

// ListPlugins 返回所有插件
func (m *Manager) ListPlugins() []plugin.Info {
	return m.registry.List()
}

// GetRegistry 返回插件注册表
func (m *Manager) GetRegistry() *plugin.Registry {
	return m.registry
}
