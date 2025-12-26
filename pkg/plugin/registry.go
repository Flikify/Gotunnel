package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Registry 管理可用的 plugins
type Registry struct {
	builtin  map[string]ProxyHandler // 内置 Go 实现
	enabled  map[string]bool         // 启用状态
	mu       sync.RWMutex
}

// NewRegistry 创建 plugin 注册表
func NewRegistry() *Registry {
	return &Registry{
		builtin: make(map[string]ProxyHandler),
		enabled: make(map[string]bool),
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
	r.enabled[meta.Name] = true // 默认启用
	return nil
}

// Get 返回指定代理类型的 handler
func (r *Registry) Get(proxyType string) (ProxyHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 先查找内置 plugin
	if handler, ok := r.builtin[proxyType]; ok {
		if !r.enabled[proxyType] {
			return nil, fmt.Errorf("plugin %s is disabled", proxyType)
		}
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
	for name, handler := range r.builtin {
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

// Enable 启用插件
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.builtin[name]; !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	r.enabled[name] = true
	return nil
}

// Disable 禁用插件
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.builtin[name]; !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	r.enabled[name] = false
	return nil
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
