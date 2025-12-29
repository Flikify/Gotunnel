package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// PluginVersionStore 插件版本存储
type PluginVersionStore struct {
	path     string
	versions map[string]string // pluginName -> version
	mu       sync.RWMutex
}

// NewPluginVersionStore 创建版本存储
func NewPluginVersionStore(dataDir string) (*PluginVersionStore, error) {
	store := &PluginVersionStore{
		path:     filepath.Join(dataDir, "plugin_versions.json"),
		versions: make(map[string]string),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// load 从文件加载版本信息
func (s *PluginVersionStore) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.versions)
}

// save 保存版本信息到文件
func (s *PluginVersionStore) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.versions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// GetVersion 获取插件版本
func (s *PluginVersionStore) GetVersion(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.versions[name]
}

// SetVersion 设置插件版本
func (s *PluginVersionStore) SetVersion(name, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.versions[name] = version
	return s.save()
}
