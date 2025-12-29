package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Registry 管理可用的 plugins
type Registry struct {
	serverPlugins map[string]ServerPlugin // 服务端插件
	clientPlugins map[string]ClientPlugin // 客户端插件
	enabled       map[string]bool         // 启用状态
	mu            sync.RWMutex
}

// NewRegistry 创建 plugin 注册表
func NewRegistry() *Registry {
	return &Registry{
		serverPlugins: make(map[string]ServerPlugin),
		clientPlugins: make(map[string]ClientPlugin),
		enabled:       make(map[string]bool),
	}
}

// RegisterServer 注册服务端插件
func (r *Registry) RegisterServer(handler ServerPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := handler.Metadata()
	if meta.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if _, exists := r.serverPlugins[meta.Name]; exists {
		return fmt.Errorf("plugin %s already registered", meta.Name)
	}

	r.serverPlugins[meta.Name] = handler
	r.enabled[meta.Name] = true
	return nil
}

// RegisterClient 注册客户端插件
func (r *Registry) RegisterClient(handler ClientPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := handler.Metadata()
	if meta.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if _, exists := r.clientPlugins[meta.Name]; exists {
		return fmt.Errorf("client plugin %s already registered", meta.Name)
	}

	r.clientPlugins[meta.Name] = handler
	r.enabled[meta.Name] = true
	return nil
}

// GetServer 返回服务端插件
func (r *Registry) GetServer(name string) (ServerPlugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if handler, ok := r.serverPlugins[name]; ok {
		if !r.enabled[name] {
			return nil, fmt.Errorf("plugin %s is disabled", name)
		}
		return handler, nil
	}
	return nil, fmt.Errorf("plugin %s not found", name)
}

// GetClient 返回客户端插件
func (r *Registry) GetClient(name string) (ClientPlugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if handler, ok := r.clientPlugins[name]; ok {
		if !r.enabled[name] {
			return nil, fmt.Errorf("client plugin %s is disabled", name)
		}
		return handler, nil
	}
	return nil, fmt.Errorf("client plugin %s not found", name)
}

// List 返回所有可用的 plugins
func (r *Registry) List() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []Info

	for name, handler := range r.serverPlugins {
		plugins = append(plugins, Info{
			Metadata: handler.Metadata(),
			Loaded:   true,
			Enabled:  r.enabled[name],
		})
	}

	for name, handler := range r.clientPlugins {
		plugins = append(plugins, Info{
			Metadata: handler.Metadata(),
			Loaded:   true,
			Enabled:  r.enabled[name],
		})
	}

	return plugins
}

// Has 检查 plugin 是否存在
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok1 := r.serverPlugins[name]
	_, ok2 := r.clientPlugins[name]
	return ok1 || ok2
}

// Close 关闭所有 plugins
func (r *Registry) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for name, handler := range r.serverPlugins {
		if err := handler.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close plugin %s: %w", name, err)
		}
	}
	for name, handler := range r.clientPlugins {
		if err := handler.Stop(); err != nil {
			lastErr = fmt.Errorf("failed to stop client plugin %s: %w", name, err)
		}
	}

	return lastErr
}

// Enable 启用插件
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.has(name) {
		return fmt.Errorf("plugin %s not found", name)
	}
	r.enabled[name] = true
	return nil
}

// Disable 禁用插件
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.has(name) {
		return fmt.Errorf("plugin %s not found", name)
	}
	r.enabled[name] = false
	return nil
}

// has 内部检查（无锁）
func (r *Registry) has(name string) bool {
	_, ok1 := r.serverPlugins[name]
	_, ok2 := r.clientPlugins[name]
	return ok1 || ok2
}

// IsEnabled 检查插件是否启用
func (r *Registry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled[name]
}

// RegisterAllServer 批量注册服务端插件
func (r *Registry) RegisterAllServer(handlers []ServerPlugin) error {
	for _, handler := range handlers {
		if err := r.RegisterServer(handler); err != nil {
			return err
		}
	}
	return nil
}
