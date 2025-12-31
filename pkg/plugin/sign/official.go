package sign

import (
	"crypto/ed25519"
	"sync"
)

// 官方固定公钥（客户端内置）
const OfficialPublicKeyBase64 = "0A0xRthj0wgPg8X8GJZ6/EnNpAUw5v7O//XLty+P5Yw="

var (
	officialPubKey     ed25519.PublicKey
	officialPubKeyOnce sync.Once
	officialPubKeyErr  error
)

// initOfficialKey 初始化官方公钥
func initOfficialKey() {
	officialPubKey, officialPubKeyErr = DecodePublicKey(OfficialPublicKeyBase64)
}

// GetOfficialPublicKey 获取官方公钥
func GetOfficialPublicKey() (ed25519.PublicKey, error) {
	officialPubKeyOnce.Do(initOfficialKey)
	return officialPubKey, officialPubKeyErr
}

// GetPublicKeyByID 根据 ID 获取公钥（兼容旧接口，忽略 keyID）
func GetPublicKeyByID(keyID string) (ed25519.PublicKey, error) {
	return GetOfficialPublicKey()
}
