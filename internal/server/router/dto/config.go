package dto

// UpdateServerConfigRequest is the config update payload.
type UpdateServerConfigRequest struct {
	Server *ServerConfigPart `json:"server"`
	Web    *WebConfigPart    `json:"web"`
}

// ServerConfigPart is the server config subset.
type ServerConfigPart struct {
	BindAddr         string `json:"bind_addr" binding:"omitempty"`
	BindPort         int    `json:"bind_port" binding:"omitempty,min=1,max=65535"`
	Token            string `json:"token" binding:"omitempty,min=8"`
	HeartbeatSec     int    `json:"heartbeat_sec" binding:"omitempty,min=1,max=300"`
	HeartbeatTimeout int    `json:"heartbeat_timeout" binding:"omitempty,min=1,max=600"`
}

// WebConfigPart is the web console config subset.
type WebConfigPart struct {
	Enabled  bool   `json:"enabled"`
	BindPort int    `json:"bind_port" binding:"omitempty,min=1,max=65535"`
	Username string `json:"username" binding:"omitempty,min=3,max=32"`
	Password string `json:"password" binding:"omitempty,min=6,max=64"`
}

// ServerConfigResponse is the config response payload.
type ServerConfigResponse struct {
	Server ServerConfigInfo `json:"server"`
	Web    WebConfigInfo    `json:"web"`
}

// ServerConfigInfo describes the server config.
type ServerConfigInfo struct {
	BindAddr         string `json:"bind_addr"`
	BindPort         int    `json:"bind_port"`
	Token            string `json:"token"`
	HeartbeatSec     int    `json:"heartbeat_sec"`
	HeartbeatTimeout int    `json:"heartbeat_timeout"`
}

// WebConfigInfo describes the web console config.
type WebConfigInfo struct {
	Enabled  bool   `json:"enabled"`
	BindPort int    `json:"bind_port"`
	Username string `json:"username"`
	Password string `json:"password"`
}
