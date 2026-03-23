package dto

// ProxyRule is the HTTP DTO for client proxy rules.
type ProxyRule struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	LocalIP      string `json:"local_ip"`
	LocalPort    int    `json:"local_port"`
	RemotePort   int    `json:"remote_port"`
	Enabled      *bool  `json:"enabled,omitempty"`
	AuthEnabled  bool   `json:"auth_enabled,omitempty"`
	AuthUsername string `json:"auth_username,omitempty"`
	AuthPassword string `json:"auth_password,omitempty"`
	PortStatus   string `json:"port_status,omitempty"`
}

// CreateClientRequest 创建客户端请求
// @Description 创建新客户端的请求体
type CreateClientRequest struct {
	ID    string      `json:"id" binding:"required,min=1,max=64" example:"client-001"`
	Rules []ProxyRule `json:"rules"`
}

// UpdateClientRequest 更新客户端请求
// @Description 更新客户端配置的请求体
type UpdateClientRequest struct {
	Nickname string      `json:"nickname" binding:"max=128" example:"My Client"`
	Rules    []ProxyRule `json:"rules"`
}

// ClientResponse 客户端详情响应
// @Description 客户端详细信息
type ClientResponse struct {
	ID            string      `json:"id" example:"client-001"`
	Nickname      string      `json:"nickname,omitempty" example:"My Client"`
	Rules         []ProxyRule `json:"rules"`
	Online        bool        `json:"online" example:"true"`
	LastPing      string      `json:"last_ping,omitempty" example:"2025-01-02T10:30:00Z"`
	LastOfflineAt int64       `json:"last_offline_at,omitempty" example:"1735785000"`
	RemoteAddr    string      `json:"remote_addr,omitempty" example:"192.168.1.100:54321"`
	OS            string      `json:"os,omitempty" example:"linux"`
	Arch          string      `json:"arch,omitempty" example:"amd64"`
	Version       string      `json:"version,omitempty" example:"1.0.0"`
}

// ClientListItem 客户端列表项
// @Description 客户端列表中的单个项目
type ClientListItem struct {
	ID            string `json:"id" example:"client-001"`
	Nickname      string `json:"nickname,omitempty" example:"My Client"`
	Online        bool   `json:"online" example:"true"`
	LastPing      string `json:"last_ping,omitempty"`
	LastOfflineAt int64  `json:"last_offline_at,omitempty"`
	RemoteAddr    string `json:"remote_addr,omitempty"`
	RuleCount     int    `json:"rule_count" example:"3"`
	OS            string `json:"os,omitempty" example:"linux"`
	Arch          string `json:"arch,omitempty" example:"amd64"`
	Version       string `json:"version,omitempty" example:"1.0.0"`
}
