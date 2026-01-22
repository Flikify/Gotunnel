package dto

// UpdateServerConfigRequest 更新服务器配置请求
// @Description 更新服务器配置
type UpdateServerConfigRequest struct {
	Server *ServerConfigPart `json:"server"`
	Web    *WebConfigPart    `json:"web"`
}

// ServerConfigPart 服务器配置部分
// @Description 隧道服务器配置
type ServerConfigPart struct {
	BindAddr         string `json:"bind_addr" binding:"omitempty"`
	BindPort         int    `json:"bind_port" binding:"omitempty,min=1,max=65535"`
	Token            string `json:"token" binding:"omitempty,min=8"`
	HeartbeatSec     int    `json:"heartbeat_sec" binding:"omitempty,min=1,max=300"`
	HeartbeatTimeout int    `json:"heartbeat_timeout" binding:"omitempty,min=1,max=600"`
}

// WebConfigPart Web 配置部分
// @Description Web 控制台配置
type WebConfigPart struct {
	Enabled  bool   `json:"enabled"`
	BindPort int    `json:"bind_port" binding:"omitempty,min=1,max=65535"`
	Username string `json:"username" binding:"omitempty,min=3,max=32"`
	Password string `json:"password" binding:"omitempty,min=6,max=64"`
}

// ServerConfigResponse 服务器配置响应
// @Description 服务器配置信息
type ServerConfigResponse struct {
	Server ServerConfigInfo `json:"server"`
	Web    WebConfigInfo    `json:"web"`
}

// ServerConfigInfo 服务器配置信息
type ServerConfigInfo struct {
	BindAddr         string `json:"bind_addr"`
	BindPort         int    `json:"bind_port"`
	Token            string `json:"token"` // 脱敏后的 token
	HeartbeatSec     int    `json:"heartbeat_sec"`
	HeartbeatTimeout int    `json:"heartbeat_timeout"`
}

// WebConfigInfo Web 配置信息
type WebConfigInfo struct {
	Enabled  bool   `json:"enabled"`
	BindPort int    `json:"bind_port"`
	Username string `json:"username"`
	Password string `json:"password"` // 显示为 ****
}
