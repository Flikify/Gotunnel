package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/plugin/builtin"
	"github.com/gotunnel/pkg/plugin/store"
	"github.com/gotunnel/pkg/plugin/wasm"
)

// Manager 服务端 plugin 管理器
type Manager struct {
	registry *plugin.Registry
	store    store.PluginStore
	runtime  *wasm.Runtime
	mu       sync.RWMutex
}

// NewManager 创建 plugin 管理器
func NewManager(pluginStore store.PluginStore) (*Manager, error) {
	ctx := context.Background()

	runtime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, fmt.Errorf("create wasm runtime: %w", err)
	}

	registry := plugin.NewRegistry()

	m := &Manager{
		registry: registry,
		store:    pluginStore,
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
// 这里只注册需要通过 plugin 系统提供的协议
func (m *Manager) registerBuiltins() error {
	// 注册 SOCKS5 plugin
	if err := m.registry.RegisterBuiltin(builtin.NewSOCKS5Plugin()); err != nil {
		return fmt.Errorf("register socks5: %w", err)
	}

	log.Println("[Plugin] Builtin plugins registered: socks5")
	return nil
}

// LoadStoredPlugins 从数据库加载所有 plugins
func (m *Manager) LoadStoredPlugins(ctx context.Context) error {
	if m.store == nil {
		return nil
	}

	plugins, err := m.store.GetAllPlugins()
	if err != nil {
		return err
	}

	for _, meta := range plugins {
		data, err := m.store.GetPluginData(meta.Name)
		if err != nil {
			log.Printf("[Plugin] Failed to load %s: %v", meta.Name, err)
			continue
		}

		if err := m.loadWASMPlugin(ctx, meta.Name, data); err != nil {
			log.Printf("[Plugin] Failed to init %s: %v", meta.Name, err)
		}
	}

	return nil
}

// loadWASMPlugin 加载 WASM plugin
func (m *Manager) loadWASMPlugin(ctx context.Context, name string, data []byte) error {
	_, err := m.runtime.LoadModule(ctx, name, data)
	if err != nil {
		return err
	}
	log.Printf("[Plugin] WASM plugin loaded: %s", name)
	return nil
}

// InstallPlugin 安装新的 WASM plugin
func (m *Manager) InstallPlugin(ctx context.Context, meta plugin.PluginMetadata, wasmData []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证 checksum
	hash := sha256.Sum256(wasmData)
	checksum := hex.EncodeToString(hash[:])
	if meta.Checksum != "" && meta.Checksum != checksum {
		return fmt.Errorf("checksum mismatch")
	}
	meta.Checksum = checksum
	meta.Size = int64(len(wasmData))

	// 存储到数据库
	if m.store != nil {
		if err := m.store.SavePlugin(meta, wasmData); err != nil {
			return err
		}
	}

	// 加载到运行时
	return m.loadWASMPlugin(ctx, meta.Name, wasmData)
}

// GetHandler 返回指定代理类型的 handler
func (m *Manager) GetHandler(proxyType string) (plugin.ProxyHandler, error) {
	return m.registry.Get(proxyType)
}

// ListPlugins 返回所有可用的 plugins
func (m *Manager) ListPlugins() []plugin.PluginInfo {
	return m.registry.List()
}

// Close 关闭管理器
func (m *Manager) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}
