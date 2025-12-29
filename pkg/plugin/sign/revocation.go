package sign

import (
	"sync"
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

// 内置撤销列表（编译时确定）
var builtinRevocations = []RevocationEntry{
	// 示例：{PluginName: "malicious-plugin", Reason: "security vulnerability"}
}

var (
	revocationCache     map[string][]RevocationEntry // pluginName -> entries
	revocationCacheOnce sync.Once
)

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
