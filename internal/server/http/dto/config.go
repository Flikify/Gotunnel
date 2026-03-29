package dto

import "github.com/gotunnel/internal/server/service"

// UpdateServerConfigRequest is the config update payload.
type UpdateServerConfigRequest struct {
	Server *ServerConfigPart `json:"server"`
	Web    *WebConfigPart    `json:"web"`
}

// ToConfigUpdate converts the HTTP payload into the service-layer change request.
func (r UpdateServerConfigRequest) ToConfigUpdate() service.ConfigUpdate {
	update := service.ConfigUpdate{}
	if r.Server != nil {
		update.Server = &service.ServerConfigUpdate{
			BindAddr:                 r.Server.BindAddr,
			BindPort:                 r.Server.BindPort,
			Token:                    r.Server.Token,
			HeartbeatSec:             r.Server.HeartbeatSec,
			HeartbeatTimeout:         r.Server.HeartbeatTimeout,
			MaxClientProxies:         r.Server.MaxClientProxies,
			ClientResponseTimeoutSec: r.Server.ClientResponseTimeoutSec,
		}
	}
	if r.Web != nil {
		update.Web = &service.WebConfigUpdate{
			Enabled:   r.Web.Enabled,
			BindPort:  r.Web.BindPort,
			Username:  r.Web.Username,
			Password:  r.Web.Password,
			CDNPrefix: r.Web.CDNPrefix,
		}
	}
	return update
}

// ServerConfigPart is the server config subset.
type ServerConfigPart struct {
	BindAddr                 string `json:"bind_addr" binding:"omitempty"`
	BindPort                 int    `json:"bind_port" binding:"omitempty,min=1,max=65535"`
	Token                    string `json:"token" binding:"omitempty,min=8"`
	HeartbeatSec             *int   `json:"heartbeat_sec" binding:"omitempty,min=1,max=3600"`
	HeartbeatTimeout         *int   `json:"heartbeat_timeout" binding:"omitempty,min=1,max=3600"`
	MaxClientProxies         *int   `json:"max_client_proxies" binding:"omitempty,gte=0,lte=10000"`
	ClientResponseTimeoutSec *int   `json:"client_response_timeout_sec" binding:"omitempty,min=1,max=300"`
}

// WebConfigPart is the web console config subset.
type WebConfigPart struct {
	Enabled   *bool   `json:"enabled"`
	BindPort  *int    `json:"bind_port" binding:"omitempty,min=1,max=65535"`
	Username  *string `json:"username" binding:"omitempty,min=3,max=32"`
	Password  *string `json:"password" binding:"omitempty,min=6,max=64"`
	CDNPrefix *string `json:"cdn_prefix" binding:"omitempty,max=256"`
}

// ServerConfigResponse is the config response payload.
type ServerConfigResponse struct {
	Server ServerConfigInfo `json:"server"`
	Web    WebConfigInfo    `json:"web"`
}

// ConfigUpdateResponse reports the persisted configuration result.
type ConfigUpdateResponse struct {
	Status                string   `json:"status"`
	AppliedRuntimeFields  []string `json:"applied_runtime_fields,omitempty"`
	RestartRequiredFields []string `json:"restart_required_fields,omitempty"`
}

// ServerConfigInfo describes the server config.
type ServerConfigInfo struct {
	BindAddr                 string `json:"bind_addr"`
	BindPort                 int    `json:"bind_port"`
	Token                    string `json:"token"`
	HeartbeatSec             int    `json:"heartbeat_sec"`
	HeartbeatTimeout         int    `json:"heartbeat_timeout"`
	MaxClientProxies         int    `json:"max_client_proxies"`
	ClientResponseTimeoutSec int    `json:"client_response_timeout_sec"`
}

// WebConfigInfo describes the web console config.
type WebConfigInfo struct {
	Enabled   bool   `json:"enabled"`
	BindPort  int    `json:"bind_port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	CDNPrefix string `json:"cdn_prefix"`
}
