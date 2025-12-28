package plugin

import (
	"net"
	"time"
)

// =============================================================================
// 核心接口定义 - 按职责分离
// =============================================================================

// Dialer 网络拨号接口（已在 types.go 中定义，此处为文档说明）
// type Dialer interface {
//     Dial(network, address string) (net.Conn, error)
// }

// PortManager 端口管理接口（仅服务端可用）
type PortManager interface {
	// ReservePort 预留端口，返回错误如果端口已被占用
	ReservePort(port int) error
	// ReleasePort 释放端口
	ReleasePort(port int)
	// IsPortAvailable 检查端口是否可用
	IsPortAvailable(port int) bool
}

// RuleManager 代理规则管理接口（仅服务端可用）
type RuleManager interface {
	// CreateRule 创建代理规则
	CreateRule(rule *RuleConfig) error
	// DeleteRule 删除代理规则
	DeleteRule(clientID, ruleName string) error
	// GetRules 获取客户端的代理规则
	GetRules(clientID string) ([]RuleConfig, error)
	// UpdateRule 更新代理规则
	UpdateRule(clientID string, rule *RuleConfig) error
}

// ClientManager 客户端管理接口（仅服务端可用）
type ClientManager interface {
	// GetClientList 获取所有客户端列表
	GetClientList() ([]ClientInfo, error)
	// IsClientOnline 检查客户端是否在线
	IsClientOnline(clientID string) bool
}

// Logger 日志接口
type Logger interface {
	// Log 记录日志
	Log(level LogLevel, format string, args ...interface{})
}

// ConfigStore 配置存储接口
type ConfigStore interface {
	// GetConfig 获取配置值
	GetConfig(key string) string
	// SetConfig 设置配置值
	SetConfig(key, value string)
}

// EventBus 事件总线接口
type EventBus interface {
	// OnEvent 订阅事件
	OnEvent(eventType EventType, handler EventHandler)
	// EmitEvent 发送事件
	EmitEvent(event *Event)
}

// =============================================================================
// 组合接口
// =============================================================================

// PluginAPI 插件 API 主接口，组合所有子接口
// 插件可以通过此接口访问 GoTunnel 的功能
type PluginAPI interface {
	// 网络操作
	Dial(network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Listen(network, address string) (net.Listener, error)

	// 端口管理（服务端）
	PortManager

	// 规则管理（服务端）
	RuleManager

	// 客户端管理（服务端）
	ClientManager

	// 日志
	Logger

	// 配置
	ConfigStore

	// 事件
	EventBus

	// 上下文
	GetContext() *Context
	GetClientID() string
	GetServerInfo() *ServerInfo
}
