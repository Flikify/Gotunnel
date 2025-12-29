package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Registry 管理可用的 plugins
type Registry struct {
	serverPlugins map[string]ProxyHandler  // 服务端插件
	clientPlugins map[string]ClientHandler // 客户端插件
	enabled       map[string]bool          // 启用状态
	mu            sync.RWMutex
}

// NewRegistry 创建 plugin 注册表
func NewRegistry() *Registry {
	return &Registry{
		serverPlugins: make(map[string]ProxyHandler),
		clientPlugins: make(map[string]ClientHandler),
		enabled:       make(map[string]bool),
	}
}

// RegisterBuiltin 注册服务端插件
func (r *Registry) RegisterBuiltin(handler ProxyHandler) error {
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

// RegisterClientPlugin 注册客户端插件
func (r *Registry) RegisterClientPlugin(handler ClientHandler) error {
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

// Get 返回指定代理类型的服务端 handler
func (r *Registry) Get(proxyType string) (ProxyHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if handler, ok := r.serverPlugins[proxyType]; ok {
		if !r.enabled[proxyType] {
			return nil, fmt.Errorf("plugin %s is disabled", proxyType)
		}
		return handler, nil
	}

	return nil, fmt.Errorf("plugin %s not found", proxyType)
}

// GetClientPlugin 返回指定类型的客户端 handler
func (r *Registry) GetClientPlugin(name string) (ClientHandler, error) {
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
func (r *Registry) List() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []PluginInfo

	// 服务端插件
	for name, handler := range r.serverPlugins {
		plugins = append(plugins, PluginInfo{
			Metadata: handler.Metadata(),
			Loaded:   true,
			Enabled:  r.enabled[name],
		})
	}

	// 客户端插件
	for name, handler := range r.clientPlugins {
		plugins = append(plugins, PluginInfo{
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

// RegisterAll 批量注册插件
func (r *Registry) RegisterAll(handlers []ProxyHandler) error {
	for _, handler := range handlers {
		if err := r.RegisterBuiltin(handler); err != nil {
			return err
		}
	}
	return nil
}
