package sqlite

import (
	"github.com/gotunnel/internal/core/client"
	corerule "github.com/gotunnel/internal/core/rule"
)

// Client re-exports the server domain client model for storage callers.
type Client = client.Client

// ClientStore 客户端存储接口
type ClientStore interface {
	GetAllClients() ([]Client, error)
	GetClient(id string) (*Client, error)
	CreateClient(c *Client) error
	UpdateClient(c *Client) error
	DeleteClient(id string) error
	ClientExists(id string) (bool, error)
	GetClientRules(id string) ([]corerule.ProxyRule, error)
	CreateInstallToken(token *InstallToken) error
	GetInstallToken(token string) (*InstallToken, error)
	MarkTokenUsed(token string) error
	DeleteExpiredTokens(expireTime int64) error
	Close() error
}

// Store 统一存储接口
type Store interface {
	ClientStore
	TrafficStore
	Close() error
}

// TrafficRecord 流量记录
type TrafficRecord struct {
	Timestamp int64 `json:"timestamp"` // Unix 时间戳（小时级别）
	Inbound   int64 `json:"inbound"`   // 入站流量（字节）
	Outbound  int64 `json:"outbound"`  // 出站流量（字节）
}

// TrafficStore 流量存储接口
type TrafficStore interface {
	AddTraffic(inbound, outbound int64) error
	GetTotalTraffic() (inbound, outbound int64, err error)
	Get24HourTraffic() (inbound, outbound int64, err error)
	GetHourlyTraffic(hours int) ([]TrafficRecord, error)
}

// InstallToken 安装token
type InstallToken struct {
	Token     string `json:"token"`
	CreatedAt int64  `json:"created_at"`
	Used      bool   `json:"used"`
}

// InstallTokenStore 安装token存储接口
type InstallTokenStore interface {
	CreateInstallToken(token *InstallToken) error
	GetInstallToken(token string) (*InstallToken, error)
	MarkTokenUsed(token string) error
	DeleteExpiredTokens(expireTime int64) error
}
