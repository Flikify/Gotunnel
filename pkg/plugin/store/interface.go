package store

import (
	"github.com/gotunnel/pkg/plugin"
)

// PluginStore 管理 plugin 持久化
type PluginStore interface {
	// GetAllPlugins 返回所有存储的 plugins
	GetAllPlugins() ([]plugin.PluginMetadata, error)

	// GetPlugin 返回指定 plugin 的元数据
	GetPlugin(name string) (*plugin.PluginMetadata, error)

	// GetPluginData 返回 WASM 二进制
	GetPluginData(name string) ([]byte, error)

	// SavePlugin 存储 plugin
	SavePlugin(metadata plugin.PluginMetadata, wasmData []byte) error

	// DeletePlugin 删除 plugin
	DeletePlugin(name string) error

	// PluginExists 检查 plugin 是否存在
	PluginExists(name string) (bool, error)

	// Close 关闭存储
	Close() error
}
