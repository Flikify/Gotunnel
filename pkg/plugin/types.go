package plugin

import (
	"net"
	"time"
)

// PluginType 定义 plugin 类别
type PluginType string

const (
	PluginTypeProxy   PluginType = "proxy"   // 代理协议插件 (SOCKS5 等)
	PluginTypeApp     PluginType = "app"     // 应用插件 (VNC, 文件管理等)
	PluginTypeService PluginType = "service" // 服务插件 (Web服务等)
	PluginTypeTool    PluginType = "tool"    // 工具插件 (监控、日志等)
)

// PluginSource 表示 plugin 来源
type PluginSource string

const (
	PluginSourceBuiltin PluginSource = "builtin" // 内置编译
	PluginSourceWASM    PluginSource = "wasm"    // WASM 模块
)

// ConfigFieldType 配置字段类型
type ConfigFieldType string

const (
	ConfigFieldString   ConfigFieldType = "string"
	ConfigFieldNumber   ConfigFieldType = "number"
	ConfigFieldBool     ConfigFieldType = "bool"
	ConfigFieldSelect   ConfigFieldType = "select"   // 下拉选择
	ConfigFieldPassword ConfigFieldType = "password" // 密码输入
)

// ConfigField 配置字段定义
type ConfigField struct {
	Key         string          `json:"key"`                   // 配置键名
	Label       string          `json:"label"`                 // 显示标签
	Type        ConfigFieldType `json:"type"`                  // 字段类型
	Default     string          `json:"default,omitempty"`     // 默认值
	Required    bool            `json:"required,omitempty"`    // 是否必填
	Options     []string        `json:"options,omitempty"`     // select 类型的选项
	Description string          `json:"description,omitempty"` // 字段描述
}

// PluginMetadata 描述一个 plugin
type PluginMetadata struct {
	Name         string            `json:"name"`                   // 唯一标识符 (如 "socks5")
	Version      string            `json:"version"`                // 语义化版本
	Type         PluginType        `json:"type"`                   // Plugin 类别
	Source       PluginSource      `json:"source"`                 // builtin 或 wasm
	Description  string            `json:"description"`            // 人类可读描述
	Author       string            `json:"author"`                 // Plugin 作者
	Icon         string            `json:"icon,omitempty"`         // 图标文件名 (如 "socks5.png")
	Checksum     string            `json:"checksum,omitempty"`     // WASM 二进制的 SHA256
	Size         int64             `json:"size,omitempty"`         // WASM 二进制大小
	Capabilities []string          `json:"capabilities,omitempty"` // 所需 host functions
	ConfigSchema []ConfigField     `json:"config_schema,omitempty"`// 配置模式定义
}

// PluginInfo 组合元数据和运行时状态
type PluginInfo struct {
	Metadata PluginMetadata `json:"metadata"`
	Loaded   bool           `json:"loaded"`
	Enabled  bool           `json:"enabled"`
	LoadedAt time.Time      `json:"loaded_at,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// Dialer 用于建立连接的接口
type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// ProxyHandler 是所有 proxy plugin 必须实现的接口
type ProxyHandler interface {
	// Metadata 返回 plugin 信息
	Metadata() PluginMetadata

	// Init 使用配置初始化 plugin
	Init(config map[string]string) error

	// HandleConn 处理传入连接
	// dialer 用于通过隧道建立连接
	HandleConn(conn net.Conn, dialer Dialer) error

	// Close 释放 plugin 资源
	Close() error
}

// LogLevel 日志级别
type LogLevel uint8

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

// ConnHandle WASM 连接句柄
type ConnHandle uint32

// HostContext 提供给 WASM plugin 的 host functions
type HostContext interface {
	// 网络操作
	Dial(network, address string) (ConnHandle, error)
	Read(handle ConnHandle, buf []byte) (int, error)
	Write(handle ConnHandle, buf []byte) (int, error)
	CloseConn(handle ConnHandle) error

	// 客户端连接操作
	ClientRead(buf []byte) (int, error)
	ClientWrite(buf []byte) (int, error)

	// 日志
	Log(level LogLevel, message string)

	// 时间
	Now() int64

	// 配置
	GetConfig(key string) string
}
