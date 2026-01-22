package handler

import (
	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/pkg/protocol"
)

// AppInterface 应用接口
type AppInterface interface {
	GetClientStore() db.ClientStore
	GetServer() ServerInterface
	GetConfig() *config.ServerConfig
	GetConfigPath() string
	SaveConfig() error
	GetJSPluginStore() db.JSPluginStore
}

// ServerInterface 服务端接口
type ServerInterface interface {
	GetClientStatus(clientID string) (online bool, lastPing, remoteAddr, clientOS, clientArch string)
	GetAllClientStatus() map[string]struct {
		Online     bool
		LastPing   string
		RemoteAddr string
		OS         string
		Arch       string
	}
	ReloadConfig() error
	GetBindAddr() string
	GetBindPort() int
	PushConfigToClient(clientID string) error
	DisconnectClient(clientID string) error
	GetPluginList() []PluginInfo
	EnablePlugin(name string) error
	DisablePlugin(name string) error
	InstallPluginsToClient(clientID string, plugins []string) error
	GetPluginConfigSchema(name string) ([]ConfigField, error)
	SyncPluginConfigToClient(clientID string, pluginName string, config map[string]string) error
	InstallJSPluginToClient(clientID string, req JSPluginInstallRequest) error
	RestartClient(clientID string) error
	StartClientPlugin(clientID, pluginID, pluginName, ruleName string) error
	StopClientPlugin(clientID, pluginID, pluginName, ruleName string) error
	RestartClientPlugin(clientID, pluginID, pluginName, ruleName string) error
	UpdateClientPluginConfig(clientID, pluginID, pluginName, ruleName string, config map[string]string, restart bool) error
	SendUpdateToClient(clientID, downloadURL string) error
	// 日志流
	StartClientLogStream(clientID, sessionID string, lines int, follow bool, level string) (<-chan protocol.LogEntry, error)
	StopClientLogStream(sessionID string)
	// 插件状态查询
	GetClientPluginStatus(clientID string) ([]protocol.PluginStatusEntry, error)
	// 插件规则管理
	StartPluginRule(clientID string, rule protocol.ProxyRule) error
	StopPluginRule(clientID string, remotePort int) error
	// 端口检查
	IsPortAvailable(port int, excludeClientID string) bool
	// 插件 API 代理
	ProxyPluginAPIRequest(clientID string, req protocol.PluginAPIRequest) (*protocol.PluginAPIResponse, error)
}

// ConfigField 配置字段
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
}

// RuleSchema 规则表单模式
type RuleSchema struct {
	NeedsLocalAddr bool          `json:"needs_local_addr"`
	ExtraFields    []ConfigField `json:"extra_fields,omitempty"`
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Source      string      `json:"source"`
	Icon        string      `json:"icon,omitempty"`
	Enabled     bool        `json:"enabled"`
	RuleSchema  *RuleSchema `json:"rule_schema,omitempty"`
}

// JSPluginInstallRequest JS 插件安装请求
type JSPluginInstallRequest struct {
	PluginID   string            `json:"plugin_id"`
	PluginName string            `json:"plugin_name"`
	Source     string            `json:"source"`
	Signature  string            `json:"signature"`
	RuleName   string            `json:"rule_name"`
	RemotePort int               `json:"remote_port"`
	Config     map[string]string `json:"config"`
	AutoStart  bool              `json:"auto_start"`
}
