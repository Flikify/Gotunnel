package wasm

import (
	"context"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Runtime 管理 wazero WASM 运行时
type Runtime struct {
	runtime wazero.Runtime
	modules map[string]*Module
	mu      sync.RWMutex
}

// NewRuntime 创建新的 WASM 运行时
func NewRuntime(ctx context.Context) (*Runtime, error) {
	r := wazero.NewRuntime(ctx)
	return &Runtime{
		runtime: r,
		modules: make(map[string]*Module),
	}, nil
}

// GetWazeroRuntime 返回底层 wazero 运行时
func (r *Runtime) GetWazeroRuntime() wazero.Runtime {
	return r.runtime
}

// LoadModule 从字节加载 WASM 模块
func (r *Runtime) LoadModule(ctx context.Context, name string, wasmBytes []byte) (*Module, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[name]; exists {
		return nil, fmt.Errorf("module %s already loaded", name)
	}

	compiled, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile module: %w", err)
	}

	module := &Module{
		name:     name,
		compiled: compiled,
	}

	r.modules[name] = module
	return module, nil
}

// GetModule 获取已加载的模块
func (r *Runtime) GetModule(name string) (*Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[name]
	return m, ok
}

// UnloadModule 卸载 WASM 模块
func (r *Runtime) UnloadModule(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	module, exists := r.modules[name]
	if !exists {
		return fmt.Errorf("module %s not found", name)
	}

	if err := module.Close(ctx); err != nil {
		return err
	}

	delete(r.modules, name)
	return nil
}

// Close 关闭运行时
func (r *Runtime) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, module := range r.modules {
		if err := module.Close(ctx); err != nil {
			return fmt.Errorf("failed to close module %s: %w", name, err)
		}
	}

	return r.runtime.Close(ctx)
}

// Module WASM 模块封装
type Module struct {
	name     string
	compiled wazero.CompiledModule
	instance api.Module
}

// Name 返回模块名称
func (m *Module) Name() string {
	return m.name
}

// Close 关闭模块
func (m *Module) Close(ctx context.Context) error {
	if m.instance != nil {
		if err := m.instance.Close(ctx); err != nil {
			return err
		}
	}
	return m.compiled.Close(ctx)
}
