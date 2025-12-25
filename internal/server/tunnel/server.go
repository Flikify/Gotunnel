package tunnel

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
	"github.com/gotunnel/pkg/utils"
	"github.com/hashicorp/yamux"
)

// Server 隧道服务端
type Server struct {
	clientStore db.ClientStore
	bindAddr    string
	bindPort    int
	token       string
	heartbeat   int
	hbTimeout   int
	portManager *utils.PortManager
	clients     map[string]*ClientSession
	mu          sync.RWMutex
}

// ClientSession 客户端会话
type ClientSession struct {
	ID        string
	Session   *yamux.Session
	Rules     []protocol.ProxyRule
	Listeners map[int]net.Listener
	LastPing  time.Time
	mu        sync.Mutex
}

// NewServer 创建服务端
func NewServer(cs db.ClientStore, bindAddr string, bindPort int, token string, heartbeat, hbTimeout int) *Server {
	return &Server{
		clientStore: cs,
		bindAddr:    bindAddr,
		bindPort:    bindPort,
		token:       token,
		heartbeat:   heartbeat,
		hbTimeout:   hbTimeout,
		portManager: utils.NewPortManager(),
		clients:     make(map[string]*ClientSession),
	}
}

// Run 启动服务端
func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.bindAddr, s.bindPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	defer ln.Close()

	log.Printf("[Server] Listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[Server] Accept error: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

// handleConnection 处理客户端连接
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		log.Printf("[Server] Read auth error: %v", err)
		return
	}

	if msg.Type != protocol.MsgTypeAuth {
		log.Printf("[Server] Expected auth, got %d", msg.Type)
		return
	}

	var authReq protocol.AuthRequest
	if err := msg.ParsePayload(&authReq); err != nil {
		log.Printf("[Server] Parse auth error: %v", err)
		return
	}

	if authReq.Token != s.token {
		s.sendAuthResponse(conn, false, "invalid token")
		return
	}

	rules, err := s.clientStore.GetClientRules(authReq.ClientID)
	if err != nil || rules == nil {
		s.sendAuthResponse(conn, false, "client not configured")
		return
	}

	conn.SetReadDeadline(time.Time{})

	if err := s.sendAuthResponse(conn, true, "ok"); err != nil {
		return
	}

	log.Printf("[Server] Client %s authenticated", authReq.ClientID)
	s.setupClientSession(conn, authReq.ClientID, rules)
}

// setupClientSession 建立客户端会话
func (s *Server) setupClientSession(conn net.Conn, clientID string, rules []protocol.ProxyRule) {
	session, err := yamux.Server(conn, nil)
	if err != nil {
		log.Printf("[Server] Yamux error: %v", err)
		return
	}

	cs := &ClientSession{
		ID:        clientID,
		Session:   session,
		Rules:     rules,
		Listeners: make(map[int]net.Listener),
		LastPing:  time.Now(),
	}

	s.registerClient(cs)
	defer s.unregisterClient(cs)

	if err := s.sendProxyConfig(session, rules); err != nil {
		log.Printf("[Server] Send config error: %v", err)
		return
	}

	s.startProxyListeners(cs)
	go s.heartbeatLoop(cs)

	<-session.CloseChan()
	log.Printf("[Server] Client %s disconnected", clientID)
}

// sendAuthResponse 发送认证响应
func (s *Server) sendAuthResponse(conn net.Conn, success bool, message string) error {
	resp := protocol.AuthResponse{Success: success, Message: message}
	msg, _ := protocol.NewMessage(protocol.MsgTypeAuthResp, resp)
	return protocol.WriteMessage(conn, msg)
}

// sendProxyConfig 发送代理配置
func (s *Server) sendProxyConfig(session *yamux.Session, rules []protocol.ProxyRule) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	cfg := protocol.ProxyConfig{Rules: rules}
	msg, _ := protocol.NewMessage(protocol.MsgTypeProxyConfig, cfg)
	return protocol.WriteMessage(stream, msg)
}

// registerClient 注册客户端
func (s *Server) registerClient(cs *ClientSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[cs.ID] = cs
}

// unregisterClient 注销客户端
func (s *Server) unregisterClient(cs *ClientSession) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cs.mu.Lock()
	for port, ln := range cs.Listeners {
		ln.Close()
		s.portManager.Release(port)
	}
	cs.mu.Unlock()

	delete(s.clients, cs.ID)
}

// startProxyListeners 启动代理监听
func (s *Server) startProxyListeners(cs *ClientSession) {
	for _, rule := range cs.Rules {
		if err := s.portManager.Reserve(rule.RemotePort, cs.ID); err != nil {
			log.Printf("[Server] Port %d error: %v", rule.RemotePort, err)
			continue
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", rule.RemotePort))
		if err != nil {
			log.Printf("[Server] Listen %d error: %v", rule.RemotePort, err)
			s.portManager.Release(rule.RemotePort)
			continue
		}

		cs.mu.Lock()
		cs.Listeners[rule.RemotePort] = ln
		cs.mu.Unlock()

		log.Printf("[Server] Proxy %s: :%d -> %s:%d",
			rule.Name, rule.RemotePort, rule.LocalIP, rule.LocalPort)

		go s.acceptProxyConns(cs, ln, rule)
	}
}

// acceptProxyConns 接受代理连接
func (s *Server) acceptProxyConns(cs *ClientSession, ln net.Listener, rule protocol.ProxyRule) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handleProxyConn(cs, conn, rule)
	}
}

// handleProxyConn 处理代理连接
func (s *Server) handleProxyConn(cs *ClientSession, conn net.Conn, rule protocol.ProxyRule) {
	defer conn.Close()

	stream, err := cs.Session.Open()
	if err != nil {
		log.Printf("[Server] Open stream error: %v", err)
		return
	}
	defer stream.Close()

	req := protocol.NewProxyRequest{RemotePort: rule.RemotePort}
	msg, _ := protocol.NewMessage(protocol.MsgTypeNewProxy, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return
	}

	relay.Relay(conn, stream)
}

// heartbeatLoop 心跳检测循环
func (s *Server) heartbeatLoop(cs *ClientSession) {
	ticker := time.NewTicker(time.Duration(s.heartbeat) * time.Second)
	defer ticker.Stop()

	timeout := time.Duration(s.hbTimeout) * time.Second

	for {
		select {
		case <-ticker.C:
			cs.mu.Lock()
			if time.Since(cs.LastPing) > timeout {
				cs.mu.Unlock()
				log.Printf("[Server] Client %s heartbeat timeout", cs.ID)
				cs.Session.Close()
				return
			}
			cs.mu.Unlock()

			stream, err := cs.Session.Open()
			if err != nil {
				return
			}
			msg := &protocol.Message{Type: protocol.MsgTypeHeartbeat}
			protocol.WriteMessage(stream, msg)
			stream.Close()

		case <-cs.Session.CloseChan():
			return
		}
	}
}

// GetClientStatus 获取客户端状态
func (s *Server) GetClientStatus(clientID string) (online bool, lastPing string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cs, ok := s.clients[clientID]; ok {
		cs.mu.Lock()
		defer cs.mu.Unlock()
		return true, cs.LastPing.Format(time.RFC3339)
	}
	return false, ""
}

// GetAllClientStatus 获取所有客户端状态
func (s *Server) GetAllClientStatus() map[string]struct {
	Online   bool
	LastPing string
} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]struct {
		Online   bool
		LastPing string
	})

	for id, cs := range s.clients {
		cs.mu.Lock()
		result[id] = struct {
			Online   bool
			LastPing string
		}{
			Online:   true,
			LastPing: cs.LastPing.Format(time.RFC3339),
		}
		cs.mu.Unlock()
	}
	return result
}

// ReloadConfig 重新加载配置
func (s *Server) ReloadConfig() error {
	return nil
}

// GetBindAddr 获取绑定地址
func (s *Server) GetBindAddr() string {
	return s.bindAddr
}

// GetBindPort 获取绑定端口
func (s *Server) GetBindPort() int {
	return s.bindPort
}
