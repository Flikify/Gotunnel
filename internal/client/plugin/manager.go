package plugin

import (
	"log"
	"sync"

	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/plugin/builtin"
)

// Manager 客户端 plugin 管理器
type Manager struct {
	registry *plugin.Registry
	mu       sync.RWMutex
}

// NewManager 创建客户端 plugin 管理器
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
	for _, h := range builtin.GetClientPlugins() {
		if err := m.registry.RegisterClient(h); err != nil {
			return err
		}
	}
	log.Printf("[Plugin] Registered %d client plugins", len(builtin.GetClientPlugins()))
	return nil
}

// GetClient 返回客户端插件
func (m *Manager) GetClient(name string) (plugin.ClientPlugin, error) {
	return m.registry.GetClient(name)
}

// GetRegistry 返回插件注册表
func (m *Manager) GetRegistry() *plugin.Registry {
	return m.registry
}
