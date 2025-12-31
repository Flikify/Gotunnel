package tunnel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// versionStoreData 版本存储数据结构（带 HMAC）
type versionStoreData struct {
	Versions map[string]string `json:"versions"`
	HMAC     string            `json:"hmac"`
}

// PluginVersionStore 插件版本存储
type PluginVersionStore struct {
	path     string
	hmacKey  []byte
	versions map[string]string // pluginName -> version
	mu       sync.RWMutex
}

// NewPluginVersionStore 创建版本存储
func NewPluginVersionStore(dataDir string) (*PluginVersionStore, error) {
	store := &PluginVersionStore{
		path:     filepath.Join(dataDir, "plugin_versions.json"),
		hmacKey:  deriveHMACKey(dataDir),
		versions: make(map[string]string),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// deriveHMACKey 从数据目录派生 HMAC 密钥
func deriveHMACKey(dataDir string) []byte {
	// 使用数据目录路径和机器特征派生密钥
	hostname, _ := os.Hostname()
	seed := fmt.Sprintf("gotunnel-version-store:%s:%s", dataDir, hostname)
	hash := sha256.Sum256([]byte(seed))
	return hash[:]
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

	var storeData versionStoreData
	if err := json.Unmarshal(data, &storeData); err != nil {
		// 尝试兼容旧格式（无 HMAC）
		if err := json.Unmarshal(data, &s.versions); err != nil {
			return fmt.Errorf("invalid version store format: %w", err)
		}
		// 迁移到新格式
		return s.save()
	}

	// 验证 HMAC
	if !s.verifyHMAC(storeData.Versions, storeData.HMAC) {
		// HMAC 验证失败，可能被篡改，重置版本信息
		s.versions = make(map[string]string)
		return fmt.Errorf("version store integrity check failed, data may be tampered")
	}

	s.versions = storeData.Versions
	return nil
}

// save 保存版本信息到文件
func (s *PluginVersionStore) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 计算 HMAC
	hmacValue := s.computeHMAC(s.versions)

	storeData := versionStoreData{
		Versions: s.versions,
		HMAC:     hmacValue,
	}

	data, err := json.MarshalIndent(storeData, "", "  ")
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

// computeHMAC 计算版本数据的 HMAC
func (s *PluginVersionStore) computeHMAC(versions map[string]string) string {
	data, _ := json.Marshal(versions)
	h := hmac.New(sha256.New, s.hmacKey)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// verifyHMAC 验证 HMAC
func (s *PluginVersionStore) verifyHMAC(versions map[string]string, expectedHMAC string) bool {
	computed := s.computeHMAC(versions)
	return hmac.Equal([]byte(computed), []byte(expectedHMAC))
}
