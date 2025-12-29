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

// RuleSchema 规则表单模式定义
type RuleSchema struct {
	NeedsLocalAddr bool          `json:"needs_local_addr"` // 是否需要本地地址
	ExtraFields    []ConfigField `json:"extra_fields,omitempty"` // 额外字段
}

// PluginMetadata 描述一个 plugin
type PluginMetadata struct {
	Name         string        `json:"name"`                    // 唯一标识符
	Version      string        `json:"version"`                 // 语义化版本
	Type         PluginType    `json:"type"`                    // Plugin 类别
	Source       PluginSource  `json:"source"`                  // builtin
	RunAt        Side          `json:"run_at"`                  // 运行位置: server 或 client
	Description  string        `json:"description"`             // 人类可读描述
	Author       string        `json:"author"`                  // Plugin 作者
	Icon         string        `json:"icon,omitempty"`          // 图标文件名
	Capabilities []string      `json:"capabilities,omitempty"`  // 所需能力
	ConfigSchema []ConfigField `json:"config_schema,omitempty"` // 插件配置模式
	RuleSchema   *RuleSchema   `json:"rule_schema,omitempty"`   // 规则表单模式
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
// 运行在服务端，处理外部连接并通过隧道转发
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

// ClientHandler 客户端插件接口
// 运行在客户端，提供本地服务（如 VNC 服务器、文件管理等）
type ClientHandler interface {
	// Metadata 返回 plugin 信息
	Metadata() PluginMetadata

	// Init 使用配置初始化 plugin
	Init(config map[string]string) error

	// Start 启动客户端服务
	// 返回服务监听的本地地址（如 "127.0.0.1:5900"）
	Start() (localAddr string, err error)

	// HandleConn 处理来自隧道的连接
	HandleConn(conn net.Conn) error

	// Stop 停止客户端服务
	Stop() error
}

// ExtendedProxyHandler 扩展的代理处理器接口
// 支持 PluginAPI 的插件应实现此接口
type ExtendedProxyHandler interface {
	ProxyHandler

	// SetAPI 设置 PluginAPI，允许插件调用系统功能
	SetAPI(api PluginAPI)
}

// LogLevel 日志级别
type LogLevel uint8

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

// =============================================================================
// API 相关类型
// =============================================================================

// Side 运行侧
type Side string

const (
	SideServer Side = "server"
	SideClient Side = "client"
)

// Context 插件运行上下文
type Context struct {
	PluginName string
	Side       Side
	ClientID   string
	Config     map[string]string
}

// ServerInfo 服务端信息
type ServerInfo struct {
	BindAddr string
	BindPort int
	Version  string
}

// RuleConfig 代理规则配置
type RuleConfig struct {
	ClientID     string            `json:"client_id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	LocalIP      string            `json:"local_ip"`
	LocalPort    int               `json:"local_port"`
	RemotePort   int               `json:"remote_port"`
	Enabled      bool              `json:"enabled"`
	PluginName   string            `json:"plugin_name,omitempty"`
	PluginConfig map[string]string `json:"plugin_config,omitempty"`
}

// ClientInfo 客户端信息
type ClientInfo struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Online   bool   `json:"online"`
	LastPing string `json:"last_ping,omitempty"`
}

// EventType 事件类型
type EventType string

const (
	EventClientConnect    EventType = "client_connect"
	EventClientDisconnect EventType = "client_disconnect"
	EventRuleCreated      EventType = "rule_created"
	EventRuleDeleted      EventType = "rule_deleted"
	EventProxyConnect     EventType = "proxy_connect"
	EventProxyDisconnect  EventType = "proxy_disconnect"
)

// Event 事件
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// EventHandler 事件处理函数
type EventHandler func(event *Event)

// =============================================================================
// 错误定义
// =============================================================================

// APIError API 错误
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

// 常见 API 错误
var (
	ErrNotSupported   = &APIError{Code: 1, Message: "operation not supported"}
	ErrClientNotFound = &APIError{Code: 2, Message: "client not found"}
	ErrPortOccupied   = &APIError{Code: 3, Message: "port already occupied"}
	ErrRuleNotFound   = &APIError{Code: 4, Message: "rule not found"}
	ErrRuleExists     = &APIError{Code: 5, Message: "rule already exists"}
	ErrNotConnected   = &APIError{Code: 6, Message: "not connected"}
	ErrInvalidConfig  = &APIError{Code: 7, Message: "invalid configuration"}
)

