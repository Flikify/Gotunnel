package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Registry 管理可用的 plugins
type Registry struct {
	builtin map[string]ProxyHandler // 内置 Go 实现
	mu      sync.RWMutex
}

// NewRegistry 创建 plugin 注册表
func NewRegistry() *Registry {
	return &Registry{
		builtin: make(map[string]ProxyHandler),
	}
}

// RegisterBuiltin 注册内置 plugin
func (r *Registry) RegisterBuiltin(handler ProxyHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := handler.Metadata()
	if meta.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if _, exists := r.builtin[meta.Name]; exists {
		return fmt.Errorf("plugin %s already registered", meta.Name)
	}

	r.builtin[meta.Name] = handler
	return nil
}

// Get 返回指定代理类型的 handler
func (r *Registry) Get(proxyType string) (ProxyHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 先查找内置 plugin
	if handler, ok := r.builtin[proxyType]; ok {
		return handler, nil
	}

	return nil, fmt.Errorf("plugin %s not found", proxyType)
}

// List 返回所有可用的 plugins
func (r *Registry) List() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []PluginInfo

	// 内置 plugins
	for _, handler := range r.builtin {
		plugins = append(plugins, PluginInfo{
			Metadata: handler.Metadata(),
			Loaded:   true,
		})
	}

	return plugins
}

// Has 检查 plugin 是否存在
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.builtin[name]
	return ok
}

// Close 关闭所有 plugins
func (r *Registry) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for name, handler := range r.builtin {
		if err := handler.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close plugin %s: %w", name, err)
		}
	}

	return lastErr
}
