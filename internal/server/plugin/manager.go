package plugin

import (
	"sync"

	"github.com/gotunnel/pkg/plugin"
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

	return m, nil
}

// ListPlugins 返回所有插件
func (m *Manager) ListPlugins() []plugin.Info {
	return m.registry.List()
}

// GetRegistry 返回插件注册表
func (m *Manager) GetRegistry() *plugin.Registry {
	return m.registry
}
