package db

import "github.com/gotunnel/pkg/protocol"

// ClientPlugin 客户端已安装的插件
type ClientPlugin struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Enabled bool              `json:"enabled"`
	Config  map[string]string `json:"config,omitempty"` // 插件配置
}

// Client 客户端数据
type Client struct {
	ID       string               `json:"id"`
	Nickname string               `json:"nickname,omitempty"`
	Rules    []protocol.ProxyRule `json:"rules"`
	Plugins  []ClientPlugin       `json:"plugins,omitempty"` // 已安装的插件
}

// JSPlugin JS 插件数据
type JSPlugin struct {
	Name        string            `json:"name"`
	Source      string            `json:"source"`
	Signature   string            `json:"signature"` // 官方签名 (Base64)
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Version     string            `json:"version,omitempty"`
	AutoPush    []string          `json:"auto_push"`
	Config      map[string]string `json:"config"`
	AutoStart   bool              `json:"auto_start"`
	Enabled     bool              `json:"enabled"`
}

// ClientStore 客户端存储接口
type ClientStore interface {
	GetAllClients() ([]Client, error)
	GetClient(id string) (*Client, error)
	CreateClient(c *Client) error
	UpdateClient(c *Client) error
	DeleteClient(id string) error
	ClientExists(id string) (bool, error)
	GetClientRules(id string) ([]protocol.ProxyRule, error)
	Close() error
}

// JSPluginStore JS 插件存储接口
type JSPluginStore interface {
	GetAllJSPlugins() ([]JSPlugin, error)
	GetJSPlugin(name string) (*JSPlugin, error)
	SaveJSPlugin(p *JSPlugin) error
	DeleteJSPlugin(name string) error
	SetJSPluginEnabled(name string, enabled bool) error
	UpdateJSPluginConfig(name string, config map[string]string) error
}

// Store 统一存储接口
type Store interface {
	ClientStore
	JSPluginStore
	Close() error
}
