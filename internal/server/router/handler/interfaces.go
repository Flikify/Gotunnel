package handler

import (
	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/pkg/protocol"
)

// AppInterface 应用接口
type AppInterface interface {
	GetClientStore() db.ClientStore
	GetServer() ServerInterface
	GetConfig() *config.ServerConfig
	GetConfigPath() string
	SaveConfig() error
	GetTrafficStore() db.TrafficStore
}

// ServerInterface 服务端接口
type ServerInterface interface {
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
	ReloadConfig() error
	GetBindAddr() string
	GetBindPort() int
	PushConfigToClient(clientID string) error
	DisconnectClient(clientID string) error
	RestartClient(clientID string) error
	SendUpdateToClient(clientID, downloadURL string) error
	// 日志流
	StartClientLogStream(clientID, sessionID string, lines int, follow bool, level string) (<-chan protocol.LogEntry, error)
	StopClientLogStream(sessionID string)
	// 端口检查
	IsPortAvailable(port int, excludeClientID string) bool
	// 系统状态
	GetClientSystemStats(clientID string) (*protocol.SystemStatsResponse, error)
	// 截图
	GetClientScreenshot(clientID string, quality int) (*protocol.ScreenshotResponse, error)
	// Shell 执行
	ExecuteClientShell(clientID, command string, timeout int) (*protocol.ShellExecuteResponse, error)
}
