package sign

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// RevocationEntry 撤销条目
type RevocationEntry struct {
	PluginName string `json:"plugin_name"`        // 插件名称
	Version    string `json:"version,omitempty"`  // 特定版本（空表示所有版本）
	Reason     string `json:"reason"`             // 撤销原因
	RevokedAt  int64  `json:"revoked_at"`         // 撤销时间戳
}

// RevocationList 撤销列表
type RevocationList struct {
	Version   int               `json:"version"`    // 列表版本
	UpdatedAt int64             `json:"updated_at"` // 更新时间
	Entries   []RevocationEntry `json:"entries"`    // 撤销条目
	Signature string            `json:"signature"`  // 列表签名
}

// 内置撤销列表（编译时确定，作为 fallback）
var builtinRevocations = []RevocationEntry{
	// 示例：{PluginName: "malicious-plugin", Reason: "security vulnerability"}
}

var (
	revocationCache     map[string][]RevocationEntry // pluginName -> entries
	revocationCacheOnce sync.Once
	revocationMu        sync.RWMutex
	currentListVersion  int
	lastFetchTime       time.Time
)

// RevocationConfig 远程撤销列表配置
type RevocationConfig struct {
	RemoteURL       string        // 远程撤销列表 URL
	FetchInterval   time.Duration // 拉取间隔
	RequestTimeout  time.Duration // 请求超时
	VerifySignature bool          // 是否验证签名
}

var defaultRevocationConfig = RevocationConfig{
	RemoteURL:       "", // 默认为空，不启用远程拉取
	FetchInterval:   1 * time.Hour,
	RequestTimeout:  10 * time.Second,
	VerifySignature: true,
}

var revocationConfig = defaultRevocationConfig

// SetRevocationConfig 设置远程撤销列表配置
func SetRevocationConfig(cfg RevocationConfig) {
	revocationMu.Lock()
	defer revocationMu.Unlock()
	revocationConfig = cfg
}

// GetRevocationConfig 获取当前配置
func GetRevocationConfig() RevocationConfig {
	revocationMu.RLock()
	defer revocationMu.RUnlock()
	return revocationConfig
}

// initRevocationCache 初始化撤销缓存
func initRevocationCache() {
	revocationCache = make(map[string][]RevocationEntry)
	for _, entry := range builtinRevocations {
		revocationCache[entry.PluginName] = append(
			revocationCache[entry.PluginName], entry)
	}
}

// IsPluginRevoked 检查插件是否被撤销
func IsPluginRevoked(name, version string) (bool, string) {
	revocationCacheOnce.Do(initRevocationCache)

	revocationMu.RLock()
	defer revocationMu.RUnlock()

	entries, ok := revocationCache[name]
	if !ok {
		return false, ""
	}

	for _, entry := range entries {
		// 空版本表示所有版本都被撤销
		if entry.Version == "" || entry.Version == version {
			return true, entry.Reason
		}
	}
	return false, ""
}

// FetchRemoteRevocationList 从远程拉取撤销列表
func FetchRemoteRevocationList() error {
	cfg := GetRevocationConfig()
	if cfg.RemoteURL == "" {
		return nil // 未配置远程 URL，跳过
	}

	// 检查是否需要刷新
	revocationMu.RLock()
	if time.Since(lastFetchTime) < cfg.FetchInterval {
		revocationMu.RUnlock()
		return nil
	}
	revocationMu.RUnlock()

	// 发起 HTTP 请求
	client := &http.Client{Timeout: cfg.RequestTimeout}
	resp, err := client.Get(cfg.RemoteURL)
	if err != nil {
		return fmt.Errorf("fetch revocation list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch revocation list: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read revocation list: %w", err)
	}

	var list RevocationList
	if err := json.Unmarshal(body, &list); err != nil {
		return fmt.Errorf("parse revocation list: %w", err)
	}

	// 验证签名
	if cfg.VerifySignature {
		if err := verifyRevocationListSignature(&list); err != nil {
			return fmt.Errorf("verify revocation list: %w", err)
		}
	}

	// 更新缓存
	return updateRevocationCache(&list)
}

// verifyRevocationListSignature 验证撤销列表签名
func verifyRevocationListSignature(list *RevocationList) error {
	if list.Signature == "" {
		return fmt.Errorf("missing signature")
	}

	// 获取官方公钥
	pubKey, err := GetOfficialPublicKey()
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}

	// 构造待签名数据（不含签名字段）
	signData := struct {
		Version   int               `json:"version"`
		UpdatedAt int64             `json:"updated_at"`
		Entries   []RevocationEntry `json:"entries"`
	}{
		Version:   list.Version,
		UpdatedAt: list.UpdatedAt,
		Entries:   list.Entries,
	}

	data, err := json.Marshal(signData)
	if err != nil {
		return fmt.Errorf("marshal sign data: %w", err)
	}

	return VerifyBase64(pubKey, data, list.Signature)
}

// updateRevocationCache 更新撤销缓存
func updateRevocationCache(list *RevocationList) error {
	revocationMu.Lock()
	defer revocationMu.Unlock()

	// 检查版本号，防止回滚攻击
	if list.Version < currentListVersion {
		return fmt.Errorf("revocation list version rollback: %d < %d", list.Version, currentListVersion)
	}

	// 重建缓存：先加载内置列表，再合并远程列表
	newCache := make(map[string][]RevocationEntry)
	for _, entry := range builtinRevocations {
		newCache[entry.PluginName] = append(newCache[entry.PluginName], entry)
	}
	for _, entry := range list.Entries {
		newCache[entry.PluginName] = append(newCache[entry.PluginName], entry)
	}

	revocationCache = newCache
	currentListVersion = list.Version
	lastFetchTime = time.Now()

	log.Printf("[Revocation] Updated to version %d with %d entries", list.Version, len(list.Entries))
	return nil
}

// StartRevocationRefresher 启动后台刷新协程
func StartRevocationRefresher(stopCh <-chan struct{}) {
	cfg := GetRevocationConfig()
	if cfg.RemoteURL == "" {
		return
	}

	// 立即执行一次
	if err := FetchRemoteRevocationList(); err != nil {
		log.Printf("[Revocation] Initial fetch failed: %v", err)
	}

	ticker := time.NewTicker(cfg.FetchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := FetchRemoteRevocationList(); err != nil {
				log.Printf("[Revocation] Refresh failed: %v", err)
			}
		case <-stopCh:
			log.Printf("[Revocation] Refresher stopped")
			return
		}
	}
}

// GetRevocationListVersion 获取当前撤销列表版本
func GetRevocationListVersion() int {
	revocationMu.RLock()
	defer revocationMu.RUnlock()
	return currentListVersion
}
