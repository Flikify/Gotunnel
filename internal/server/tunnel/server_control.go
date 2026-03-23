package tunnel

import (
	"fmt"
	"net"
	"time"

	"github.com/gotunnel/internal/server/domain"
	"github.com/gotunnel/pkg/protocol"
	"github.com/hashicorp/yamux"
)

func (s *Server) sendAuthResponse(conn net.Conn, success bool, message, clientID string) error {
	return s.channel.sendAuthResponse(conn, success, message, clientID)
}

func withStreamDeadline(stream net.Conn, timeout time.Duration, fn func() error) error {
	if timeout > 0 {
		if err := stream.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
		defer stream.SetDeadline(time.Time{})
	}
	return fn()
}

func (s *Server) validateProxyRuleLimit(rules []domain.ProxyRule) error {
	limit := s.maxClientProxies()
	if limit <= 0 {
		return nil
	}
	if len(rules) <= limit {
		return nil
	}
	return fmt.Errorf("client has %d proxy rules, exceeding the configured limit of %d", len(rules), limit)
}

func (s *Server) requestProxyOpen(stream net.Conn, remotePort int) error {
	return s.channel.requestProxyOpen(stream, remotePort)
}

// sendProxyConfig 发送代理配置并等待客户端确认
func (s *Server) sendProxyConfig(session *yamux.Session, rules []domain.ProxyRule) error {
	return s.channel.sendProxyConfig(session, rules)
}

func toProtocolRules(rules []domain.ProxyRule) []protocol.ProxyRule {
	if len(rules) == 0 {
		return nil
	}

	out := make([]protocol.ProxyRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, protocol.ProxyRule{
			Name:         rule.Name,
			Type:         rule.Type,
			LocalIP:      rule.LocalIP,
			LocalPort:    rule.LocalPort,
			RemotePort:   rule.RemotePort,
			Enabled:      rule.Enabled,
			AuthEnabled:  rule.AuthEnabled,
			AuthUsername: rule.AuthUsername,
			AuthPassword: rule.AuthPassword,
			PortStatus:   rule.PortStatus,
		})
	}
	return out
}

// registerClient 注册客户端
func (s *Server) registerClient(cs *ClientSession) {
	s.control.registerClient(cs)
}

func (s *Server) persistClientConnectionInfo(clientID, remoteAddr, clientOS, clientArch, clientVersion string, lastOfflineAt int64) {
	s.control.persistClientConnectionInfo(clientID, remoteAddr, clientOS, clientArch, clientVersion, lastOfflineAt)
}

// unregisterClient 注销客户端
func (s *Server) unregisterClient(cs *ClientSession) {
	s.control.unregisterClient(cs)
}

// stopProxyListeners 停止代理监听
func (s *Server) stopProxyListeners(clientID string) {
	s.proxies.stop(clientID)
}

// GetClientStatus 获取客户端状态
func (s *Server) GetClientStatus(clientID string) (online bool, lastPing, remoteAddr, clientName, clientOS, clientArch, clientVersion string) {
	status, ok := s.control.status(clientID)
	if ok {
		return true, status.LastPing, status.RemoteAddr, status.Name, status.OS, status.Arch, status.Version
	}
	return false, "", "", "", "", "", ""
}

// IsClientOnline 检查客户端是否在线
func (s *Server) IsClientOnline(clientID string) bool {
	return s.control.isClientOnline(clientID)
}

// GetAllClientStatus 获取所有客户端状态
func (s *Server) GetAllClientStatus() map[string]struct {
	Online     bool
	LastPing   string
	RemoteAddr string
	Name       string
	OS         string
	Arch       string
	Version    string
} {
	result := make(map[string]struct {
		Online     bool
		LastPing   string
		RemoteAddr string
		Name       string
		OS         string
		Arch       string
		Version    string
	})

	for clientID, status := range s.control.allStatus() {
		result[clientID] = struct {
			Online     bool
			LastPing   string
			RemoteAddr string
			Name       string
			OS         string
			Arch       string
			Version    string
		}{
			Online:     true,
			LastPing:   status.LastPing,
			RemoteAddr: status.RemoteAddr,
			Name:       status.Name,
			OS:         status.OS,
			Arch:       status.Arch,
			Version:    status.Version,
		}
	}
	return result
}

// PushConfigToClient 推送配置到客户端
func (s *Server) PushConfigToClient(clientID string) error {
	return s.control.pushConfig(clientID)
}

// DisconnectClient 断开客户端连接
func (s *Server) DisconnectClient(clientID string) error {
	return s.control.disconnectClient(clientID)
}

// shouldPushToClient 检查是否应推送到指定客户端
func (s *Server) shouldPushToClient(autoPush []string, clientID string) bool {
	if len(autoPush) == 0 {
		return true
	}
	for _, id := range autoPush {
		if id == clientID || id == "*" {
			return true
		}
	}
	return false
}

// RestartClient 重启客户端（通过断开连接，让客户端自动重连）
func (s *Server) RestartClient(clientID string) error {
	return s.control.restartClient(clientID)
}

// IsPortAvailable 检查端口是否可用
func (s *Server) IsPortAvailable(port int, excludeClientID string) bool {
	return s.control.isPortAvailable(port, excludeClientID)
}

// SendUpdateToClient 发送更新命令到客户端
func (s *Server) SendUpdateToClient(clientID, downloadURL string) error {
	return s.control.sendUpdate(clientID, downloadURL)
}
