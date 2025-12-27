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

// PluginData 插件数据
type PluginData struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Icon        string `json:"icon"`
	Checksum    string `json:"checksum"`
	Size        int64  `json:"size"`
	Enabled     bool   `json:"enabled"`
	WASMData    []byte `json:"-"`
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

// PluginStore 插件存储接口
type PluginStore interface {
	GetAllPlugins() ([]PluginData, error)
	GetPlugin(name string) (*PluginData, error)
	SavePlugin(p *PluginData) error
	DeletePlugin(name string) error
	SetPluginEnabled(name string, enabled bool) error
	GetPluginWASM(name string) ([]byte, error)
}

// Store 统一存储接口
type Store interface {
	ClientStore
	PluginStore
	Close() error
}
