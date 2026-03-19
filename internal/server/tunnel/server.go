package tunnel

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/proxy"
	"github.com/gotunnel/pkg/relay"
	"github.com/gotunnel/pkg/security"
	"github.com/gotunnel/pkg/utils"
	"github.com/hashicorp/yamux"
)

// 服务端常量
const (
	authTimeout      = 10 * time.Second
	heartbeatTimeout = 10 * time.Second
	udpBufferSize    = 65535
	maxConnections   = 10000 // 最大连接数
)

// 客户端 ID 验证正则
var clientIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// isValidClientID 验证客户端 ID 格式
func isValidClientID(id string) bool {
	return clientIDRegex.MatchString(id)
}

// Server 隧道服务端
type Server struct {
	clientStore  db.ClientStore
	trafficStore db.TrafficStore // 流量存储
	bindAddr     string
	bindPort     int
	token        string
	heartbeat    int
	hbTimeout    int
	portManager  *utils.PortManager
	clients      map[string]*ClientSession
	mu           sync.RWMutex
	tlsConfig    *tls.Config
	connSem      chan struct{}      // 连接数信号量
	activeConns  int64              // 当前活跃连接数
	listener     net.Listener       // 主监听器
	shutdown     chan struct{}      // 关闭信号
	wg           sync.WaitGroup     // 等待所有连接关闭
	logSessions  *LogSessionManager // 日志会话管理器
}

// ClientSession 客户端会话
type ClientSession struct {
	ID         string
	Name       string // 客户端名称（主机名）
	RemoteAddr string // 客户端 IP 地址
	OS         string // 客户端操作系统
	Arch       string // 客户端架构
	Version    string // 客户端版本
	Session    *yamux.Session
	Rules      []protocol.ProxyRule
	Listeners  map[int]net.Listener
	UDPConns   map[int]*net.UDPConn // UDP 连接
	LastPing   time.Time
	mu         sync.Mutex
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
		connSem:     make(chan struct{}, maxConnections),
		shutdown:    make(chan struct{}),
		logSessions: NewLogSessionManager(),
	}
}

// SetTLSConfig 设置 TLS 配置
func (s *Server) SetTLSConfig(config *tls.Config) {
	s.tlsConfig = config
}

// Shutdown 优雅关闭服务端
func (s *Server) Shutdown(timeout time.Duration) error {
	log.Printf("[Server] Initiating graceful shutdown...")
	close(s.shutdown)

	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有客户端会话
	s.mu.Lock()
	for _, cs := range s.clients {
		cs.Session.Close()
	}
	s.mu.Unlock()

	// 等待所有连接关闭，带超时
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[Server] All connections closed gracefully")
		return nil
	case <-time.After(timeout):
		log.Printf("[Server] Shutdown timeout, forcing close")
		return fmt.Errorf("shutdown timeout")
	}
}

// SetTrafficStore 设置流量存储
func (s *Server) SetTrafficStore(store db.TrafficStore) {
	s.trafficStore = store
}

// Run 启动服务端
func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.bindAddr, s.bindPort)

	var ln net.Listener
	var err error

	if s.tlsConfig != nil {
		ln, err = tls.Listen("tcp", addr, s.tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to listen TLS on %s: %v", addr, err)
		}
		log.Printf("[Server] TLS listening on %s", addr)
	} else {
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %v", addr, err)
		}
		log.Printf("[Server] Listening on %s (no TLS)", addr)
	}
	s.listener = ln

	for {
		select {
		case <-s.shutdown:
			log.Printf("[Server] Shutdown signal received, stopping accept loop")
			ln.Close()
			return nil
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return nil
			default:
				log.Printf("[Server] Accept error: %v", err)
				continue
			}
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// handleConnection 处理客户端连接
func (s *Server) handleConnection(conn net.Conn) {
	clientIP := conn.RemoteAddr().String()

	// 连接数限制检查
	select {
	case s.connSem <- struct{}{}:
		defer func() { <-s.connSem }()
	default:
		security.LogConnRejected(clientIP, "max connections reached")
		conn.Close()
		return
	}

	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(authTimeout))

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

	// 验证token：支持常规token和一次性安装token
	validToken := authReq.Token == s.token
	var isInstallToken bool

	if !validToken {
		// 尝试验证安装token
		if tokenStore, ok := s.clientStore.(db.InstallTokenStore); ok {
			if installToken, err := tokenStore.GetInstallToken(authReq.Token); err == nil {
				if !installToken.Used && time.Now().Unix()-installToken.CreatedAt < 3600 {
					// token有效且未过期
					validToken = true
					isInstallToken = true
					// 验证客户端ID匹配
					// 使用token中的客户端ID
				}
			}
		}
	}

	if !validToken {
		security.LogInvalidToken(clientIP)
		s.sendAuthResponse(conn, false, "invalid token", "")
		return
	}

	// 处理客户端 ID
	clientID := authReq.ClientID
	if clientID == "" || !isValidClientID(clientID) {
		security.LogInvalidClientID(clientIP, clientID)
		s.sendAuthResponse(conn, false, "invalid client id format", "")
		return
	}

	// 检查客户端是否存在，不存在则自动创建
	exists, err := s.clientStore.ClientExists(clientID)
	if err != nil || !exists {
		newClient := &db.Client{ID: clientID, Nickname: authReq.Name, Rules: []protocol.ProxyRule{}}
		if err := s.clientStore.CreateClient(newClient); err != nil {
			log.Printf("[Server] Create client error: %v", err)
			s.sendAuthResponse(conn, false, "failed to create client", "")
			return
		}
		log.Printf("[Server] New client registered: %s (%s)", clientID, authReq.Name)
	} else if authReq.Name != "" {
		// 客户端已存在，仅当 Nickname 为空时才用客户端名称更新
		// 这样服务端手动设置的名称不会被客户端覆盖
		if client, err := s.clientStore.GetClient(clientID); err == nil {
			if client.Nickname == "" {
				client.Nickname = authReq.Name
				s.clientStore.UpdateClient(client)
			}
		}
	}

	rules, _ := s.clientStore.GetClientRules(clientID)
	if rules == nil {
		rules = []protocol.ProxyRule{}
	}

	// 如果使用安装token，标记为已使用
	if isInstallToken {
		if tokenStore, ok := s.clientStore.(db.InstallTokenStore); ok {
			tokenStore.MarkTokenUsed(authReq.Token)
		}
	}

	conn.SetReadDeadline(time.Time{})

	if err := s.sendAuthResponse(conn, true, "ok", clientID); err != nil {
		return
	}

	security.LogAuthSuccess(clientIP, clientID)
	s.setupClientSession(conn, clientID, authReq.Name, authReq.OS, authReq.Arch, authReq.Version, rules)
}

// setupClientSession 建立客户端会话
func (s *Server) setupClientSession(conn net.Conn, clientID, clientName, clientOS, clientArch, clientVersion string, rules []protocol.ProxyRule) {
	session, err := yamux.Server(conn, nil)
	if err != nil {
		log.Printf("[Server] Yamux error: %v", err)
		return
	}

	// 提取客户端 IP（去掉端口）
	remoteAddr := conn.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = host
	}

	cs := &ClientSession{
		ID:         clientID,
		Name:       clientName,
		RemoteAddr: remoteAddr,
		OS:         clientOS,
		Arch:       clientArch,
		Version:    clientVersion,
		Session:    session,
		Rules:      rules,
		Listeners:  make(map[int]net.Listener),
		UDPConns:   make(map[int]*net.UDPConn),
		LastPing:   time.Now(),
	}

	s.registerClient(cs)
	defer s.unregisterClient(cs)

	// 启动代理监听器（会更新 rules 的 PortStatus）
	s.startProxyListeners(cs)

	// 发送配置到客户端（包含端口状态）
	if err := s.sendProxyConfig(session, cs.Rules); err != nil {
		log.Printf("[Server] Send config error: %v", err)
		return
	}

	go s.heartbeatLoop(cs)

	<-session.CloseChan()
	log.Printf("[Server] Client %s disconnected", clientID)
}

// sendAuthResponse 发送认证响应
func (s *Server) sendAuthResponse(conn net.Conn, success bool, message, clientID string) error {
	resp := protocol.AuthResponse{Success: success, Message: message, ClientID: clientID}
	msg, err := protocol.NewMessage(protocol.MsgTypeAuthResp, resp)
	if err != nil {
		return err
	}
	return protocol.WriteMessage(conn, msg)
}

// sendProxyConfig 发送代理配置并等待客户端确认
func (s *Server) sendProxyConfig(session *yamux.Session, rules []protocol.ProxyRule) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	cfg := protocol.ProxyConfig{Rules: rules}
	msg, err := protocol.NewMessage(protocol.MsgTypeProxyConfig, cfg)
	if err != nil {
		return err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	// 等待客户端确认
	ack, err := protocol.ReadMessage(stream)
	if err != nil {
		return fmt.Errorf("wait config ack: %w", err)
	}
	if ack.Type != protocol.MsgTypeProxyReady {
		return fmt.Errorf("unexpected ack type: %d", ack.Type)
	}

	return nil
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
	for port, conn := range cs.UDPConns {
		conn.Close()
		s.portManager.Release(port)
	}
	cs.mu.Unlock()

	delete(s.clients, cs.ID)
}

// stopProxyListeners 停止代理监听
func (s *Server) stopProxyListeners(cs *ClientSession) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 关闭 TCP 监听器
	for port, ln := range cs.Listeners {
		ln.Close()
		s.portManager.Release(port)
	}
	cs.Listeners = make(map[int]net.Listener)

	// 关闭 UDP 连接
	for port, conn := range cs.UDPConns {
		conn.Close()
		s.portManager.Release(port)
	}
	cs.UDPConns = make(map[int]*net.UDPConn)
}

// startProxyListeners 启动代理监听
func (s *Server) startProxyListeners(cs *ClientSession) {
	for i := range cs.Rules {
		rule := &cs.Rules[i]
		if !rule.IsEnabled() {
			continue
		}

		ruleType := rule.Type
		if ruleType == "" {
			ruleType = "tcp"
		}

		// UDP 单独处理
		if ruleType == "udp" {
			s.startUDPListener(cs, rule)
			continue
		}

		// TCP 类型
		if err := s.portManager.Reserve(rule.RemotePort, cs.ID); err != nil {
			log.Printf("[Server] Port %d error: %v", rule.RemotePort, err)
			rule.PortStatus = "failed: " + err.Error()
			continue
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", rule.RemotePort))
		if err != nil {
			log.Printf("[Server] Listen %d error: %v", rule.RemotePort, err)
			s.portManager.Release(rule.RemotePort)
			rule.PortStatus = "failed: " + err.Error()
			continue
		}

		rule.PortStatus = "listening"

		cs.mu.Lock()
		cs.Listeners[rule.RemotePort] = ln
		cs.mu.Unlock()

		switch ruleType {
		case "socks5":
			log.Printf("[Server] SOCKS5 proxy %s on :%d", rule.Name, rule.RemotePort)
			go s.acceptProxyServerConns(cs, ln, *rule)
		case "http", "https":
			log.Printf("[Server] HTTP proxy %s on :%d", rule.Name, rule.RemotePort)
			go s.acceptProxyServerConns(cs, ln, *rule)
		case "websocket":
			log.Printf("[Server] Websocket proxy %s on :%d", rule.Name, rule.RemotePort)
			go s.acceptWebsocketConns(cs, ln, *rule)
		default:
			log.Printf("[Server] TCP proxy %s: :%d -> %s:%d",
				rule.Name, rule.RemotePort, rule.LocalIP, rule.LocalPort)
			go s.acceptProxyConns(cs, ln, *rule)
		}
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

// acceptProxyServerConns 接受 SOCKS5/HTTP 代理连接
func (s *Server) acceptProxyServerConns(cs *ClientSession, ln net.Listener, rule protocol.ProxyRule) {
	dialer := proxy.NewTunnelDialer(cs.Session)

	// 使用内置 proxy 实现 (带流量统计和认证)
	username := ""
	password := ""
	if rule.AuthEnabled {
		username = rule.AuthUsername
		password = rule.AuthPassword
	}
	proxyServer := proxy.NewServer(rule.Type, dialer, s.recordTraffic, username, password)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go proxyServer.HandleConn(conn)
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

	relay.RelayWithStats(conn, stream, s.recordTraffic)
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

			// 发送心跳并等待响应
			if s.sendHeartbeat(cs) {
				cs.mu.Lock()
				cs.LastPing = time.Now()
				cs.mu.Unlock()
			}

		case <-cs.Session.CloseChan():
			return
		}
	}
}

// sendHeartbeat 发送心跳并等待响应
func (s *Server) sendHeartbeat(cs *ClientSession) bool {
	stream, err := cs.Session.Open()
	if err != nil {
		return false
	}
	defer stream.Close()

	// 设置读写超时
	stream.SetDeadline(time.Now().Add(heartbeatTimeout))

	msg := &protocol.Message{Type: protocol.MsgTypeHeartbeat}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return false
	}

	// 等待心跳响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return false
	}

	return resp.Type == protocol.MsgTypeHeartbeatAck
}

// GetClientStatus 获取客户端状态
func (s *Server) GetClientStatus(clientID string) (online bool, lastPing, remoteAddr, clientName, clientOS, clientArch, clientVersion string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cs, ok := s.clients[clientID]; ok {
		cs.mu.Lock()
		defer cs.mu.Unlock()
		return true, cs.LastPing.Format(time.RFC3339), cs.RemoteAddr, cs.Name, cs.OS, cs.Arch, cs.Version
	}
	return false, "", "", "", "", "", ""
}

// IsClientOnline 检查客户端是否在线
func (s *Server) IsClientOnline(clientID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.clients[clientID]
	return ok
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
	// 先复制客户端引用，避免嵌套锁
	s.mu.RLock()
	clients := make([]*ClientSession, 0, len(s.clients))
	for _, cs := range s.clients {
		clients = append(clients, cs)
	}
	s.mu.RUnlock()

	result := make(map[string]struct {
		Online     bool
		LastPing   string
		RemoteAddr string
		Name       string
		OS         string
		Arch       string
		Version    string
	})

	for _, cs := range clients {
		cs.mu.Lock()
		result[cs.ID] = struct {
			Online     bool
			LastPing   string
			RemoteAddr string
			Name       string
			OS         string
			Arch       string
			Version    string
		}{
			Online:     true,
			LastPing:   cs.LastPing.Format(time.RFC3339),
			RemoteAddr: cs.RemoteAddr,
			Name:       cs.Name,
			OS:         cs.OS,
			Arch:       cs.Arch,
			Version:    cs.Version,
		}
		cs.mu.Unlock()
	}
	return result
}

// ReloadConfig 重新加载配置
// 注意: 当前版本不支持热重载，需要重启服务
func (s *Server) ReloadConfig() error {
	return fmt.Errorf("hot reload not supported, please restart the server")
}

// GetBindAddr 获取绑定地址
func (s *Server) GetBindAddr() string {
	return s.bindAddr
}

// GetBindPort 获取绑定端口
func (s *Server) GetBindPort() int {
	return s.bindPort
}

// PushConfigToClient 推送配置到客户端
func (s *Server) PushConfigToClient(clientID string) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	rules, err := s.clientStore.GetClientRules(clientID)
	if err != nil {
		return err
	}

	// 停止旧的监听器
	s.stopProxyListeners(cs)

	// 更新规则
	cs.mu.Lock()
	cs.Rules = rules
	cs.mu.Unlock()

	// 启动新的监听器
	s.startProxyListeners(cs)

	// 检查是否有端口启动失败
	var failedPorts []string
	for _, rule := range cs.Rules {
		if rule.IsEnabled() && strings.HasPrefix(rule.PortStatus, "failed:") {
			failedPorts = append(failedPorts, fmt.Sprintf("port %d: %s", rule.RemotePort, strings.TrimPrefix(rule.PortStatus, "failed: ")))
		}
	}

	// 发送配置到客户端
	if err := s.sendProxyConfig(cs.Session, cs.Rules); err != nil {
		return err
	}

	// 如果有端口失败，返回错误
	if len(failedPorts) > 0 {
		return fmt.Errorf("some ports failed to start: %s", strings.Join(failedPorts, "; "))
	}

	return nil
}

// DisconnectClient 断开客户端连接
func (s *Server) DisconnectClient(clientID string) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	return cs.Session.Close()
}

// startUDPListener 启动 UDP 监听
func (s *Server) startUDPListener(cs *ClientSession, rule *protocol.ProxyRule) {
	if err := s.portManager.Reserve(rule.RemotePort, cs.ID); err != nil {
		log.Printf("[Server] UDP port %d error: %v", rule.RemotePort, err)
		rule.PortStatus = "failed: " + err.Error()
		return
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", rule.RemotePort))
	if err != nil {
		log.Printf("[Server] UDP resolve error: %v", err)
		s.portManager.Release(rule.RemotePort)
		rule.PortStatus = "failed: " + err.Error()
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("[Server] UDP listen %d error: %v", rule.RemotePort, err)
		s.portManager.Release(rule.RemotePort)
		rule.PortStatus = "failed: " + err.Error()
		return
	}

	rule.PortStatus = "listening"

	cs.mu.Lock()
	cs.UDPConns[rule.RemotePort] = conn
	cs.mu.Unlock()

	log.Printf("[Server] UDP proxy %s: :%d -> %s:%d",
		rule.Name, rule.RemotePort, rule.LocalIP, rule.LocalPort)

	go s.handleUDPConn(cs, conn, *rule)
}

// handleUDPConn 处理 UDP 连接
func (s *Server) handleUDPConn(cs *ClientSession, conn *net.UDPConn, rule protocol.ProxyRule) {
	buf := make([]byte, udpBufferSize)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		// 封装 UDP 数据包发送到客户端
		packet := protocol.UDPPacket{
			RemotePort: rule.RemotePort,
			ClientAddr: clientAddr.String(),
			Data:       buf[:n],
		}

		go s.sendUDPPacket(cs, conn, clientAddr, packet)
	}
}

// sendUDPPacket 发送 UDP 数据包到客户端
func (s *Server) sendUDPPacket(cs *ClientSession, conn *net.UDPConn, clientAddr *net.UDPAddr, packet protocol.UDPPacket) {
	stream, err := cs.Session.Open()
	if err != nil {
		return
	}
	defer stream.Close()

	msg, err := protocol.NewMessage(protocol.MsgTypeUDPData, packet)
	if err != nil {
		return
	}

	if err := protocol.WriteMessage(stream, msg); err != nil {
		return
	}

	// 记录入站流量 (从外部接收的数据)
	s.recordTraffic(int64(len(packet.Data)), 0)

	// 等待客户端响应
	respMsg, err := protocol.ReadMessage(stream)
	if err != nil {
		return
	}

	if respMsg.Type == protocol.MsgTypeUDPData {
		var respPacket protocol.UDPPacket
		if err := respMsg.ParsePayload(&respPacket); err != nil {
			return
		}
		conn.WriteToUDP(respPacket.Data, clientAddr)
		// 记录出站流量 (发送回外部的数据)
		s.recordTraffic(0, int64(len(respPacket.Data)))
	}
}

// checkHTTPBasicAuth 检查 HTTP Basic Auth
// 返回 (认证成功, 已读取的数据)
func (s *Server) checkHTTPBasicAuth(conn net.Conn, username, password string) (bool, []byte) {
	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // 重置超时

	// 读取 HTTP 请求头
	buf := make([]byte, 8192) // 增大缓冲区以处理更大的请求头
	n, err := conn.Read(buf)
	if err != nil {
		return false, nil
	}

	data := buf[:n]
	request := string(data)

	// 解析 Authorization 头
	authHeader := ""
	lines := strings.Split(request, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "authorization:") {
			authHeader = strings.TrimSpace(line[14:])
			break
		}
	}

	// 检查 Basic Auth
	if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
		s.sendHTTPUnauthorized(conn)
		return false, nil
	}

	// 解码 Base64
	encoded := authHeader[6:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		s.sendHTTPUnauthorized(conn)
		return false, nil
	}

	// 解析 username:password
	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		s.sendHTTPUnauthorized(conn)
		return false, nil
	}

	if parts[0] != username || parts[1] != password {
		s.sendHTTPUnauthorized(conn)
		return false, nil
	}

	return true, data
}

// sendHTTPUnauthorized 发送 401 未授权响应
func (s *Server) sendHTTPUnauthorized(conn net.Conn) {
	response := "HTTP/1.1 401 Unauthorized\r\n" +
		"WWW-Authenticate: Basic realm=\"GoTunnel Plugin\"\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 12\r\n" +
		"\r\n" +
		"Unauthorized"
	conn.Write([]byte(response))
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
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	// 发送重启消息
	stream, err := cs.Session.Open()
	if err != nil {
		return err
	}

	req := protocol.ClientRestartRequest{
		Reason: "server requested restart",
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeClientRestart, req)
	protocol.WriteMessage(stream, msg)
	stream.Close()

	// 等待一小段时间后断开连接
	time.AfterFunc(100*time.Millisecond, func() {
		cs.Session.Close()
	})

	log.Printf("[Server] Restart initiated for client %s", clientID)
	return nil
}

// IsPortAvailable 检查端口是否可用
func (s *Server) IsPortAvailable(port int, excludeClientID string) bool {
	// 检查系统端口
	if !utils.IsPortAvailable(port) {
		return false
	}
	// 检查是否被其他客户端占用
	s.mu.RLock()
	defer s.mu.RUnlock()
	for clientID, cs := range s.clients {
		if clientID == excludeClientID {
			continue
		}
		cs.mu.Lock()
		_, occupied := cs.Listeners[port]
		cs.mu.Unlock()
		if occupied {
			return false
		}
	}
	return true
}

// SendUpdateToClient 发送更新命令到客户端
func (s *Server) SendUpdateToClient(clientID, downloadURL string) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	// 发送更新消息
	stream, err := cs.Session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.UpdateDownloadRequest{
		DownloadURL: downloadURL,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeUpdateDownload, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	log.Printf("[Server] Update command sent to client %s: %s", clientID, downloadURL)
	return nil
}

// StartClientLogStream 启动客户端日志流
func (s *Server) StartClientLogStream(clientID, sessionID string, lines int, follow bool, level string) (<-chan protocol.LogEntry, error) {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("client %s not found or not online", clientID)
	}

	// 打开到客户端的流
	stream, err := cs.Session.Open()
	if err != nil {
		return nil, err
	}

	// 发送日志请求
	req := protocol.LogRequest{
		SessionID: sessionID,
		Lines:     lines,
		Follow:    follow,
		Level:     level,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeLogRequest, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		stream.Close()
		return nil, err
	}

	// 创建会话
	session := s.logSessions.CreateSession(clientID, sessionID, stream)
	listener := session.AddListener()

	// 启动 goroutine 读取客户端日志
	go s.readClientLogs(session, stream)

	return listener, nil
}

// readClientLogs 读取客户端日志并广播到监听器
func (s *Server) readClientLogs(session *LogSession, stream net.Conn) {
	defer s.logSessions.RemoveSession(session.ID)

	for {
		msg, err := protocol.ReadMessage(stream)
		if err != nil {
			return
		}

		if msg.Type != protocol.MsgTypeLogData {
			continue
		}

		var data protocol.LogData
		if err := msg.ParsePayload(&data); err != nil {
			continue
		}

		for _, entry := range data.Entries {
			session.Broadcast(entry)
		}

		if data.EOF {
			return
		}
	}
}

// StopClientLogStream 停止客户端日志流
func (s *Server) StopClientLogStream(sessionID string) {
	session := s.logSessions.GetSession(sessionID)
	if session == nil {
		return
	}

	// 发送停止请求到客户端
	s.mu.RLock()
	cs, ok := s.clients[session.ClientID]
	s.mu.RUnlock()

	if ok {
		stream, err := cs.Session.Open()
		if err == nil {
			req := protocol.LogStopRequest{SessionID: sessionID}
			msg, _ := protocol.NewMessage(protocol.MsgTypeLogStop, req)
			protocol.WriteMessage(stream, msg)
			stream.Close()
		}
	}

	s.logSessions.RemoveSession(sessionID)
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

// boolPtr 返回 bool 值的指针
func boolPtr(b bool) *bool {
	return &b
}

// GetClientSystemStats 获取客户端系统状态
func (s *Server) GetClientSystemStats(clientID string) (*protocol.SystemStatsResponse, error) {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("client %s not online", clientID)
	}

	stream, err := cs.Session.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// 设置超时
	stream.SetDeadline(time.Now().Add(10 * time.Second))

	// 发送请求
	msg, _ := protocol.NewMessage(protocol.MsgTypeSystemStatsRequest, nil)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return nil, err
	}

	// 读取响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, err
	}

	if resp.Type != protocol.MsgTypeSystemStatsResponse {
		return nil, fmt.Errorf("unexpected response type: %d", resp.Type)
	}

	var stats protocol.SystemStatsResponse
	if err := resp.ParsePayload(&stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// GetClientScreenshot 获取客户端截图
func (s *Server) GetClientScreenshot(clientID string, quality int) (*protocol.ScreenshotResponse, error) {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("client %s not online", clientID)
	}

	stream, err := cs.Session.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// 设置超时
	stream.SetDeadline(time.Now().Add(15 * time.Second))

	// 发送请求
	req := protocol.ScreenshotRequest{Quality: quality}
	msg, _ := protocol.NewMessage(protocol.MsgTypeScreenshotRequest, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return nil, err
	}

	// 读取响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, err
	}

	if resp.Type != protocol.MsgTypeScreenshotResponse {
		return nil, fmt.Errorf("unexpected response type: %d", resp.Type)
	}

	var screenshot protocol.ScreenshotResponse
	if err := resp.ParsePayload(&screenshot); err != nil {
		return nil, err
	}

	if screenshot.Error != "" {
		return nil, fmt.Errorf("screenshot failed: %s", screenshot.Error)
	}

	return &screenshot, nil
}

// ExecuteClientShell 执行客户端 Shell 命令
func (s *Server) ExecuteClientShell(clientID, command string, timeout int) (*protocol.ShellExecuteResponse, error) {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("client %s not online", clientID)
	}

	stream, err := cs.Session.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// 设置超时 (比命令超时长一点)
	if timeout <= 0 {
		timeout = 30
	}
	stream.SetDeadline(time.Now().Add(time.Duration(timeout+5) * time.Second))

	// 发送请求
	req := protocol.ShellExecuteRequest{
		Command: command,
		Timeout: timeout,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeShellExecuteRequest, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return nil, err
	}

	// 读取响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, err
	}

	if resp.Type != protocol.MsgTypeShellExecuteResponse {
		return nil, fmt.Errorf("unexpected response type: %d", resp.Type)
	}

	var result protocol.ShellExecuteResponse
	if err := resp.ParsePayload(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
