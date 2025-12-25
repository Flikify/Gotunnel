package db

import "github.com/gotunnel/pkg/protocol"

// Client 客户端数据
type Client struct {
	ID    string               `json:"id"`
	Rules []protocol.ProxyRule `json:"rules"`
}

// ClientStore 客户端存储接口
type ClientStore interface {
	// GetAllClients 获取所有客户端
	GetAllClients() ([]Client, error)

	// GetClient 获取单个客户端
	GetClient(id string) (*Client, error)

	// CreateClient 创建客户端
	CreateClient(c *Client) error

	// UpdateClient 更新客户端
	UpdateClient(c *Client) error

	// DeleteClient 删除客户端
	DeleteClient(id string) error

	// ClientExists 检查客户端是否存在
	ClientExists(id string) (bool, error)

	// GetClientRules 获取客户端规则
	GetClientRules(id string) ([]protocol.ProxyRule, error)

	// Close 关闭连接
	Close() error
}
