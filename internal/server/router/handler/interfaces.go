package handler

import "github.com/gotunnel/internal/server/service"

// ClientRuntimeInterface covers client status and control operations used by ClientHandler.
type ClientRuntimeInterface interface {
	IsClientOnline(clientID string) bool
	GetClientStatus(clientID string) (online bool, lastPing, remoteAddr, clientName, clientOS, clientArch, clientVersion string)
	GetAllClientStatus() map[string]struct {
		Online     bool
		LastPing   string
		RemoteAddr string
		Name       string
		OS         string
		Arch       string
		Version    string
	}
	PushConfigToClient(clientID string) error
	DisconnectClient(clientID string) error
	RestartClient(clientID string) error
}

// UpdateRuntimeInterface covers client update delivery.
type UpdateRuntimeInterface interface {
	SendUpdateToClient(clientID, downloadURL string) error
}

// ServerInfoInterface provides readonly server bind information.
type ServerInfoInterface interface {
	GetBindAddr() string
	GetBindPort() int
}

// ServerInterface 服务端接口
type ServerInterface interface {
	ClientRuntimeInterface
	UpdateRuntimeInterface
	ServerInfoInterface
	service.RemoteOpsRuntime
	ApplyRuntimeConfig(heartbeatSec, heartbeatTimeoutSec, maxClientProxies, clientResponseTimeoutSec int)
	// 端口检查
	IsPortAvailable(port int, excludeClientID string) bool
}
