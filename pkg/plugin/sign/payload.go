package sign

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PluginPayload 插件签名载荷
type PluginPayload struct {
	Name       string `json:"name"`        // 插件名称
	Version    string `json:"version"`     // 版本号
	SourceHash string `json:"source_hash"` // 源码 SHA256
	KeyID      string `json:"key_id"`      // 签名密钥 ID
	Timestamp  int64  `json:"timestamp"`   // 签名时间戳
}

// SignedPlugin 已签名的插件
type SignedPlugin struct {
	Payload   PluginPayload `json:"payload"`
	Signature string        `json:"signature"` // Base64 签名
}

// NormalizeSource 规范化源码（统一换行符）
func NormalizeSource(source string) string {
	// 统一换行符为 LF
	normalized := strings.ReplaceAll(source, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	// 去除尾部空白
	normalized = strings.TrimRight(normalized, " \t\n")
	return normalized
}

// HashSource 计算源码哈希
func HashSource(source string) string {
	normalized := NormalizeSource(source)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// CreatePayload 创建签名载荷
func CreatePayload(name, version, source, keyID string) *PluginPayload {
	return &PluginPayload{
		Name:       name,
		Version:    version,
		SourceHash: HashSource(source),
		KeyID:      keyID,
		Timestamp:  time.Now().Unix(),
	}
}

// SignPlugin 签名插件
func SignPlugin(priv ed25519.PrivateKey, payload *PluginPayload) (*SignedPlugin, error) {
	// 序列化载荷
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	// 签名
	sig := SignBase64(priv, data)

	return &SignedPlugin{
		Payload:   *payload,
		Signature: sig,
	}, nil
}

// VerifyPlugin 验证插件签名
func VerifyPlugin(pub ed25519.PublicKey, signed *SignedPlugin, source string) error {
	// 验证源码哈希
	expectedHash := HashSource(source)
	if signed.Payload.SourceHash != expectedHash {
		return fmt.Errorf("source hash mismatch")
	}

	// 序列化载荷
	data, err := json.Marshal(signed.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// 验证签名
	return VerifyBase64(pub, data, signed.Signature)
}

// EncodeSignedPlugin 编码已签名插件为 JSON
func EncodeSignedPlugin(sp *SignedPlugin) (string, error) {
	data, err := json.Marshal(sp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DecodeSignedPlugin 从 JSON 解码已签名插件
func DecodeSignedPlugin(data string) (*SignedPlugin, error) {
	var sp SignedPlugin
	if err := json.Unmarshal([]byte(data), &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}
