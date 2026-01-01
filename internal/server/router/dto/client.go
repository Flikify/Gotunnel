package dto

import (
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/pkg/protocol"
)

// CreateClientRequest 创建客户端请求
// @Description 创建新客户端的请求体
type CreateClientRequest struct {
	ID    string               `json:"id" binding:"required,min=1,max=64" example:"client-001"`
	Rules []protocol.ProxyRule `json:"rules"`
}

// UpdateClientRequest 更新客户端请求
// @Description 更新客户端配置的请求体
type UpdateClientRequest struct {
	Nickname string               `json:"nickname" binding:"max=128" example:"My Client"`
	Rules    []protocol.ProxyRule `json:"rules"`
	Plugins  []db.ClientPlugin    `json:"plugins"`
}

// ClientResponse 客户端详情响应
// @Description 客户端详细信息
type ClientResponse struct {
	ID         string               `json:"id" example:"client-001"`
	Nickname   string               `json:"nickname,omitempty" example:"My Client"`
	Rules      []protocol.ProxyRule `json:"rules"`
	Plugins    []db.ClientPlugin    `json:"plugins,omitempty"`
	Online     bool                 `json:"online" example:"true"`
	LastPing   string               `json:"last_ping,omitempty" example:"2025-01-02T10:30:00Z"`
	RemoteAddr string               `json:"remote_addr,omitempty" example:"192.168.1.100:54321"`
}

// ClientListItem 客户端列表项
// @Description 客户端列表中的单个项目
type ClientListItem struct {
	ID         string `json:"id" example:"client-001"`
	Nickname   string `json:"nickname,omitempty" example:"My Client"`
	Online     bool   `json:"online" example:"true"`
	LastPing   string `json:"last_ping,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	RuleCount  int    `json:"rule_count" example:"3"`
}

// InstallPluginsRequest 安装插件到客户端请求
// @Description 安装插件到指定客户端
type InstallPluginsRequest struct {
	Plugins []string `json:"plugins" binding:"required,min=1,dive,required" example:"socks5,http-proxy"`
}

// ClientPluginActionRequest 客户端插件操作请求
// @Description 对客户端插件执行操作
type ClientPluginActionRequest struct {
	RuleName string            `json:"rule_name"`
	Config   map[string]string `json:"config,omitempty"`
	Restart  bool              `json:"restart"`
}
