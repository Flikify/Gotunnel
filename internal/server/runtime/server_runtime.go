package runtime

import (
	"crypto/tls"
	"log"
	"net"
	"time"

	db "github.com/gotunnel/internal/server/storage/sqlite"
)

// SetTLSConfig 设置 TLS 配置
func (s *Server) SetTLSConfig(config *tls.Config) {
	s.tlsConfig = config
}

// ApplyRuntimeConfig 更新可热生效的运行时配置。
func (s *Server) ApplyRuntimeConfig(heartbeatSec, heartbeatTimeoutSec, maxClientProxies, clientResponseTimeoutSec int) {
	if heartbeatSec <= 0 {
		heartbeatSec = 30
	}
	if heartbeatTimeoutSec <= 0 {
		heartbeatTimeoutSec = 90
	}
	if heartbeatTimeoutSec < heartbeatSec {
		heartbeatTimeoutSec = heartbeatSec
	}
	if clientResponseTimeoutSec <= 0 {
		clientResponseTimeoutSec = 15
	}
	if maxClientProxies < 0 {
		maxClientProxies = 0
	}

	s.configMu.Lock()
	s.heartbeatSec = heartbeatSec
	s.hbTimeoutSec = heartbeatTimeoutSec
	s.maxProxies = maxClientProxies
	s.respTimeout = clientResponseTimeoutSec
	s.configMu.Unlock()
}

func (s *Server) runtimeConfig() (heartbeatSec, heartbeatTimeoutSec, maxClientProxies int, responseTimeout time.Duration) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.heartbeatSec, s.hbTimeoutSec, s.maxProxies, time.Duration(s.respTimeout) * time.Second
}

func (s *Server) clientResponseTimeout() time.Duration {
	_, _, _, timeout := s.runtimeConfig()
	return timeout
}

func (s *Server) maxClientProxies() int {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.maxProxies
}

// ClientResponseTimeout returns the configured client RPC timeout.
func (s *Server) ClientResponseTimeout() time.Duration {
	return s.clientResponseTimeout()
}

// OpenClientStream opens a control stream to an online client.
func (s *Server) OpenClientStream(clientID string) (net.Conn, error) {
	return s.sessions.openStream(clientID)
}

// Shutdown 优雅关闭服务端
func (s *Server) Shutdown(timeout time.Duration) error {
	return s.listenerLoop.shutdownGracefully(timeout, s.sessions.disconnectAll)
}

// SetTrafficStore 设置流量存储
func (s *Server) SetTrafficStore(store db.TrafficStore) {
	s.trafficStore = store
}

// GetBindAddr 获取绑定地址
func (s *Server) GetBindAddr() string {
	return s.bindAddr
}

// GetBindPort 获取绑定端口
func (s *Server) GetBindPort() int {
	return s.bindPort
}

// recordTraffic 记录流量统计
func (s *Server) recordTraffic(inbound, outbound int64) {
	if s.trafficStore == nil {
		return
	}
	if err := s.trafficStore.AddTraffic(inbound, outbound); err != nil {
		log.Printf("[Server] Record traffic error: %v", err)
	}
}
