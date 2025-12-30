package sign

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// KeyEntry 密钥条目
type KeyEntry struct {
	ID        string    `json:"id"`
	PublicKey string    `json:"public_key"`
	ValidFrom time.Time `json:"valid_from"`
	RevokedAt time.Time `json:"revoked_at,omitempty"`
}

// 官方公钥列表（支持密钥轮换）
var officialKeys = []KeyEntry{
	{
		ID:        "official-v1",
		PublicKey: "0A0xRthj0wgPg8X8GJZ6/EnNpAUw5v7O//XLty+P5Yw=",
		ValidFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	},
	// 添加新密钥时，在此处追加
}

var (
	keyCache     map[string]ed25519.PublicKey
	keyCacheOnce sync.Once
	keyMu        sync.RWMutex
	remoteKeys   []KeyEntry
)

// KeyListConfig 远程公钥列表配置
type KeyListConfig struct {
	RemoteURL      string
	FetchInterval  time.Duration
	RequestTimeout time.Duration
}

var keyListConfig = KeyListConfig{
	RemoteURL:      "",
	FetchInterval:  24 * time.Hour,
	RequestTimeout: 10 * time.Second,
}

// SetKeyListConfig 设置远程公钥列表配置
func SetKeyListConfig(cfg KeyListConfig) {
	keyMu.Lock()
	defer keyMu.Unlock()
	keyListConfig = cfg
}

// initKeyCache 初始化密钥缓存
func initKeyCache() {
	keyCache = make(map[string]ed25519.PublicKey)
	for _, entry := range officialKeys {
		if pub, err := DecodePublicKey(entry.PublicKey); err == nil {
			keyCache[entry.ID] = pub
		}
	}
}

// GetOfficialPublicKey 获取默认官方公钥（兼容旧接口）
func GetOfficialPublicKey() (ed25519.PublicKey, error) {
	return GetPublicKeyByID("official-v1")
}

// GetPublicKeyByID 根据 ID 获取公钥
func GetPublicKeyByID(keyID string) (ed25519.PublicKey, error) {
	keyCacheOnce.Do(initKeyCache)

	keyMu.RLock()
	pub, ok := keyCache[keyID]
	keyMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown key ID: %s", keyID)
	}
	return pub, nil
}

// IsKeyRevoked 检查密钥是否已吊销
func IsKeyRevoked(keyID string) bool {
	// 先检查内置密钥
	for _, entry := range officialKeys {
		if entry.ID == keyID {
			return !entry.RevokedAt.IsZero()
		}
	}
	// 再检查远程密钥
	keyMu.RLock()
	defer keyMu.RUnlock()
	for _, entry := range remoteKeys {
		if entry.ID == keyID {
			return !entry.RevokedAt.IsZero()
		}
	}
	return true // 未知密钥视为已吊销
}

// GetKeyEntry 获取密钥条目
func GetKeyEntry(keyID string) *KeyEntry {
	for i := range officialKeys {
		if officialKeys[i].ID == keyID {
			return &officialKeys[i]
		}
	}
	keyMu.RLock()
	defer keyMu.RUnlock()
	for i := range remoteKeys {
		if remoteKeys[i].ID == keyID {
			return &remoteKeys[i]
		}
	}
	return nil
}

// KeyList 远程公钥列表结构
type KeyList struct {
	Version   int        `json:"version"`
	UpdatedAt int64      `json:"updated_at"`
	Keys      []KeyEntry `json:"keys"`
	Signature string     `json:"signature"`
}

// FetchRemoteKeyList 从远程拉取公钥列表
func FetchRemoteKeyList() error {
	keyMu.RLock()
	cfg := keyListConfig
	keyMu.RUnlock()

	if cfg.RemoteURL == "" {
		return nil
	}

	client := &http.Client{Timeout: cfg.RequestTimeout}
	resp, err := client.Get(cfg.RemoteURL)
	if err != nil {
		return fmt.Errorf("fetch key list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch key list: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read key list: %w", err)
	}

	var list KeyList
	if err := json.Unmarshal(body, &list); err != nil {
		return fmt.Errorf("parse key list: %w", err)
	}

	// 验证签名（使用内置公钥验证）
	if err := verifyKeyListSignature(&list); err != nil {
		return fmt.Errorf("verify key list: %w", err)
	}

	return updateKeyCache(&list)
}

// verifyKeyListSignature 验证公钥列表签名
func verifyKeyListSignature(list *KeyList) error {
	if list.Signature == "" {
		return fmt.Errorf("missing signature")
	}

	// 使用内置公钥验证（必须用内置密钥签名）
	pubKey, err := GetOfficialPublicKey()
	if err != nil {
		return err
	}

	signData := struct {
		Version   int        `json:"version"`
		UpdatedAt int64      `json:"updated_at"`
		Keys      []KeyEntry `json:"keys"`
	}{
		Version:   list.Version,
		UpdatedAt: list.UpdatedAt,
		Keys:      list.Keys,
	}

	data, err := json.Marshal(signData)
	if err != nil {
		return err
	}

	return VerifyBase64(pubKey, data, list.Signature)
}

// updateKeyCache 更新公钥缓存
func updateKeyCache(list *KeyList) error {
	keyMu.Lock()
	defer keyMu.Unlock()

	// 保存远程密钥列表
	remoteKeys = list.Keys

	// 更新缓存
	for _, entry := range list.Keys {
		if pub, err := DecodePublicKey(entry.PublicKey); err == nil {
			keyCache[entry.ID] = pub
		}
	}

	log.Printf("[KeyList] Updated with %d keys", len(list.Keys))
	return nil
}
