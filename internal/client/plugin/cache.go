package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotunnel/pkg/plugin"
)

// CachedPlugin 缓存的 plugin 信息
type CachedPlugin struct {
	Metadata plugin.PluginMetadata
	Path     string
	LoadedAt time.Time
}

// Cache 管理本地 plugin 存储
type Cache struct {
	dir     string
	plugins map[string]*CachedPlugin
	mu      sync.RWMutex
}

// NewCache 创建 plugin 缓存
func NewCache(cacheDir string) (*Cache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	return &Cache{
		dir:     cacheDir,
		plugins: make(map[string]*CachedPlugin),
	}, nil
}

// Get 返回缓存的 plugin（如果有效）
func (c *Cache) Get(name, version, checksum string) (*CachedPlugin, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.plugins[name]
	if !ok {
		return nil, nil
	}

	// 验证版本和 checksum
	if cached.Metadata.Version != version {
		return nil, nil
	}
	if checksum != "" && cached.Metadata.Checksum != checksum {
		return nil, nil
	}

	return cached, nil
}

// Store 保存 plugin 到缓存
func (c *Cache) Store(meta plugin.PluginMetadata, wasmData []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 验证 checksum
	hash := sha256.Sum256(wasmData)
	checksum := hex.EncodeToString(hash[:])
	if meta.Checksum != "" && meta.Checksum != checksum {
		return fmt.Errorf("checksum mismatch")
	}
	meta.Checksum = checksum

	// 写入文件
	path := filepath.Join(c.dir, meta.Name+".wasm")
	if err := os.WriteFile(path, wasmData, 0644); err != nil {
		return err
	}

	c.plugins[meta.Name] = &CachedPlugin{
		Metadata: meta,
		Path:     path,
		LoadedAt: time.Now(),
	}
	return nil
}

// Remove 删除缓存的 plugin
func (c *Cache) Remove(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached, ok := c.plugins[name]
	if !ok {
		return nil
	}

	os.Remove(cached.Path)
	delete(c.plugins, name)
	return nil
}

// List 返回所有缓存的 plugins
func (c *Cache) List() []plugin.PluginMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []plugin.PluginMetadata
	for _, cached := range c.plugins {
		result = append(result, cached.Metadata)
	}
	return result
}
