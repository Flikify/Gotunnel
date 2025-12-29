package sign

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"
)

// KeyEntry 密钥条目
type KeyEntry struct {
	ID        string    // 密钥 ID
	PublicKey string    // Base64 编码的公钥
	ValidFrom time.Time // 生效时间
	RevokedAt time.Time // 吊销时间（零值表示未吊销）
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
)

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

	pub, ok := keyCache[keyID]
	if !ok {
		return nil, fmt.Errorf("unknown key ID: %s", keyID)
	}
	return pub, nil
}

// IsKeyRevoked 检查密钥是否已吊销
func IsKeyRevoked(keyID string) bool {
	for _, entry := range officialKeys {
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
	return nil
}
