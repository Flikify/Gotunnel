package plugin

import (
	"context"
	"log"
	"sync"

	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/plugin/builtin"
	"github.com/gotunnel/pkg/plugin/wasm"
)

// Manager 客户端 plugin 管理器
type Manager struct {
	registry *plugin.Registry
	cache    *Cache
	runtime  *wasm.Runtime
	mu       sync.RWMutex
}

// NewManager 创建客户端 plugin 管理器
func NewManager(cacheDir string) (*Manager, error) {
	ctx := context.Background()

	cache, err := NewCache(cacheDir)
	if err != nil {
		return nil, err
	}

	runtime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, err
	}

	registry := plugin.NewRegistry()

	m := &Manager{
		registry: registry,
		cache:    cache,
		runtime:  runtime,
	}

	// 注册内置 plugins
	if err := m.registerBuiltins(); err != nil {
		return nil, err
	}

	return m, nil
}

// registerBuiltins 注册内置 plugins
// 注意: tcp, udp, http, https 是内置类型，直接在 tunnel 中处理
func (m *Manager) registerBuiltins() error {
	// 使用统一的插件注册入口
	if err := m.registry.RegisterAll(builtin.GetAll()); err != nil {
		return err
	}
	log.Printf("[Plugin] Registered %d builtin plugins", len(builtin.GetAll()))
	return nil
}

// GetHandler 返回指定代理类型的 handler
func (m *Manager) GetHandler(proxyType string) (plugin.ProxyHandler, error) {
	return m.registry.Get(proxyType)
}

// Close 关闭管理器
func (m *Manager) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}
