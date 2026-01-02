package plugin

import (
	"net"
	"time"
)

// =============================================================================
// 基础类型
// =============================================================================

// Side 运行侧
type Side string

const (
	SideClient Side = "client"
)

// PluginType 插件类别
type PluginType string

const (
	PluginTypeProxy PluginType = "proxy" // 代理协议 (SOCKS5 等)
	PluginTypeApp   PluginType = "app"   // 应用服务 (VNC, Echo 等)
)

// PluginSource 插件来源
type PluginSource string

const (
	PluginSourceBuiltin PluginSource = "builtin" // 内置编译
	PluginSourceScript  PluginSource = "script"  // 脚本插件
)

// =============================================================================
// 配置相关
// =============================================================================

// ConfigFieldType 配置字段类型
type ConfigFieldType string

const (
	ConfigFieldString   ConfigFieldType = "string"
	ConfigFieldNumber   ConfigFieldType = "number"
	ConfigFieldBool     ConfigFieldType = "bool"
	ConfigFieldSelect   ConfigFieldType = "select"
	ConfigFieldPassword ConfigFieldType = "password"
)

// ConfigField 配置字段定义
type ConfigField struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Type        ConfigFieldType `json:"type"`
	Default     string          `json:"default,omitempty"`
	Required    bool            `json:"required,omitempty"`
	Options     []string        `json:"options,omitempty"`
	Description string          `json:"description,omitempty"`
}

// RuleSchema 规则表单模式
type RuleSchema struct {
	NeedsLocalAddr bool          `json:"needs_local_addr"`
	ExtraFields    []ConfigField `json:"extra_fields,omitempty"`
}

// =============================================================================
// 元数据
// =============================================================================

// Metadata 插件元数据
type Metadata struct {
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	Type         PluginType    `json:"type"`
	Source       PluginSource  `json:"source"`
	RunAt        Side          `json:"run_at"`
	Description  string        `json:"description"`
	Author       string        `json:"author,omitempty"`
	ConfigSchema []ConfigField `json:"config_schema,omitempty"`
	RuleSchema   *RuleSchema   `json:"rule_schema,omitempty"`
}

// Info 插件运行时信息
type Info struct {
	Metadata Metadata  `json:"metadata"`
	Loaded   bool      `json:"loaded"`
	Enabled  bool      `json:"enabled"`
	LoadedAt time.Time `json:"loaded_at,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// =============================================================================
// 核心接口
// =============================================================================

// Dialer 网络拨号接口
type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// ClientPlugin 客户端插件接口
// 运行在客户端，提供本地服务
type ClientPlugin interface {
	Metadata() Metadata
	Init(config map[string]string) error
	Start() (localAddr string, err error)
	HandleConn(conn net.Conn) error
	Stop() error
}
