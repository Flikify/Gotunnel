package tunnel

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router"
	"github.com/gotunnel/pkg/plugin"
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

// generateClientID 生成随机客户端 ID
func generateClientID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Server 隧道服务端
type Server struct {
	clientStore    db.ClientStore
	jsPluginStore  db.JSPluginStore // JS 插件存储
	bindAddr       string
	bindPort       int
	token          string
	heartbeat      int
	hbTimeout      int
	portManager    *utils.PortManager
	clients        map[string]*ClientSession
	mu             sync.RWMutex
	tlsConfig      *tls.Config
	pluginRegistry *plugin.Registry
	jsPlugins      []JSPluginEntry // 配置的 JS 插件
	connSem        chan struct{}   // 连接数信号量
	activeConns    int64           // 当前活跃连接数
	listener       net.Listener    // 主监听器
	shutdown       chan struct{}   // 关闭信号
	wg             sync.WaitGroup  // 等待所有连接关闭
	logSessions    *LogSessionManager // 日志会话管理器
}

// JSPluginEntry JS 插件条目
type JSPluginEntry struct {
	Name      string
	Source    string
	Signature string
	AutoPush  []string
	Config    map[string]string
	AutoStart bool
}

// ClientSession 客户端会话
type ClientSession struct {
	ID          string
	RemoteAddr  string // 客户端 IP 地址
	Session     *yamux.Session
	Rules       []protocol.ProxyRule
	Listeners   map[int]net.Listener
	UDPConns    map[int]*net.UDPConn // UDP 连接
	LastPing    time.Time
	mu          sync.Mutex
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

// SetPluginRegistry 设置插件注册表
func (s *Server) SetPluginRegistry(registry *plugin.Registry) {
	s.pluginRegistry = registry
}

// SetJSPluginStore 设置 JS 插件存储
func (s *Server) SetJSPluginStore(store db.JSPluginStore) {
	s.jsPluginStore = store
}

// LoadJSPlugins 加载 JS 插件配置
func (s *Server) LoadJSPlugins(plugins []JSPluginEntry) {
	s.jsPlugins = plugins
	log.Printf("[Server] Loaded %d JS plugin configs", len(plugins))
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

	if authReq.Token != s.token {
		security.LogInvalidToken(clientIP)
		s.sendAuthResponse(conn, false, "invalid token", "")
		return
	}

	// 处理客户端 ID
	clientID := authReq.ClientID
	if clientID == "" {
		clientID = generateClientID()
	} else if !isValidClientID(clientID) {
		security.LogInvalidClientID(clientIP, clientID)
		s.sendAuthResponse(conn, false, "invalid client id format", "")
		return
	}

	// 检查客户端是否存在，不存在则自动创建
	exists, err := s.clientStore.ClientExists(clientID)
	if err != nil || !exists {
		newClient := &db.Client{ID: clientID, Rules: []protocol.ProxyRule{}}
		if err := s.clientStore.CreateClient(newClient); err != nil {
			log.Printf("[Server] Create client error: %v", err)
			s.sendAuthResponse(conn, false, "failed to create client", "")
			return
		}
		log.Printf("[Server] New client registered: %s", clientID)
	}

	rules, _ := s.clientStore.GetClientRules(clientID)
	if rules == nil {
		rules = []protocol.ProxyRule{}
	}

	conn.SetReadDeadline(time.Time{})

	if err := s.sendAuthResponse(conn, true, "ok", clientID); err != nil {
		return
	}

	security.LogAuthSuccess(clientIP, clientID)
	s.setupClientSession(conn, clientID, rules)
}

// setupClientSession 建立客户端会话
func (s *Server) setupClientSession(conn net.Conn, clientID string, rules []protocol.ProxyRule) {
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
		RemoteAddr: remoteAddr,
		Session:    session,
		Rules:      rules,
		Listeners:  make(map[int]net.Listener),
		UDPConns:   make(map[int]*net.UDPConn),
		LastPing:   time.Now(),
	}

	s.registerClient(cs)
	defer s.unregisterClient(cs)

	if err := s.sendProxyConfig(session, rules); err != nil {
		log.Printf("[Server] Send config error: %v", err)
		return
	}

	// 自动推送 JS 插件
	s.autoPushJSPlugins(cs)

	s.startProxyListeners(cs)
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

// sendProxyConfig 发送代理配置
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
	for _, rule := range cs.Rules {
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

		// 检查是否为客户端插件
		if s.isClientPlugin(ruleType) {
			s.startClientPluginListener(cs, rule)
			continue
		}

		// TCP 类型
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

		switch ruleType {
		case "socks5":
			log.Printf("[Server] SOCKS5 proxy %s on :%d", rule.Name, rule.RemotePort)
			go s.acceptProxyServerConns(cs, ln, rule)
		case "http", "https":
			log.Printf("[Server] HTTP proxy %s on :%d", rule.Name, rule.RemotePort)
			go s.acceptProxyServerConns(cs, ln, rule)
		default:
			log.Printf("[Server] TCP proxy %s: :%d -> %s:%d",
				rule.Name, rule.RemotePort, rule.LocalIP, rule.LocalPort)
			go s.acceptProxyConns(cs, ln, rule)
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

	// 使用内置 proxy 实现
	proxyServer := proxy.NewServer(rule.Type, dialer)
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
func (s *Server) GetClientStatus(clientID string) (online bool, lastPing string, remoteAddr string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cs, ok := s.clients[clientID]; ok {
		cs.mu.Lock()
		defer cs.mu.Unlock()
		return true, cs.LastPing.Format(time.RFC3339), cs.RemoteAddr
	}
	return false, "", ""
}

// GetClientPluginStatus 获取客户端插件运行状态
func (s *Server) GetClientPluginStatus(clientID string) ([]protocol.PluginStatusEntry, error) {
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

	// 发送查询请求
	msg, err := protocol.NewMessage(protocol.MsgTypePluginStatusQuery, nil)
	if err != nil {
		return nil, err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return nil, err
	}

	// 读取响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, err
	}

	if resp.Type != protocol.MsgTypePluginStatusQueryResp {
		return nil, fmt.Errorf("unexpected response type: %d", resp.Type)
	}

	var statusResp protocol.PluginStatusQueryResponse
	if err := resp.ParsePayload(&statusResp); err != nil {
		return nil, err
	}

	return statusResp.Plugins, nil
}

// GetAllClientStatus 获取所有客户端状态
func (s *Server) GetAllClientStatus() map[string]struct {
	Online     bool
	LastPing   string
	RemoteAddr string
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
	})

	for _, cs := range clients {
		cs.mu.Lock()
		result[cs.ID] = struct {
			Online     bool
			LastPing   string
			RemoteAddr string
		}{
			Online:     true,
			LastPing:   cs.LastPing.Format(time.RFC3339),
			RemoteAddr: cs.RemoteAddr,
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

	// 发送配置到客户端
	return s.sendProxyConfig(cs.Session, rules)
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

// GetPluginList 获取插件列表
func (s *Server) GetPluginList() []router.PluginInfo {
	var result []router.PluginInfo

	if s.pluginRegistry == nil {
		return result
	}

	for _, info := range s.pluginRegistry.List() {
		pi := router.PluginInfo{
			Name:        info.Metadata.Name,
			Version:     info.Metadata.Version,
			Type:        string(info.Metadata.Type),
			Description: info.Metadata.Description,
			Source:      string(info.Metadata.Source),
			Enabled:     info.Enabled,
		}

		// 转换 RuleSchema
		if info.Metadata.RuleSchema != nil {
			rs := &router.RuleSchema{
				NeedsLocalAddr: info.Metadata.RuleSchema.NeedsLocalAddr,
			}
			for _, f := range info.Metadata.RuleSchema.ExtraFields {
				rs.ExtraFields = append(rs.ExtraFields, router.ConfigField{
					Key:         f.Key,
					Label:       f.Label,
					Type:        string(f.Type),
					Default:     f.Default,
					Required:    f.Required,
					Options:     f.Options,
					Description: f.Description,
				})
			}
			pi.RuleSchema = rs
		}

		result = append(result, pi)
	}
	return result
}

// EnablePlugin 启用插件
func (s *Server) EnablePlugin(name string) error {
	if s.pluginRegistry == nil {
		return fmt.Errorf("plugin registry not initialized")
	}
	return s.pluginRegistry.Enable(name)
}

// DisablePlugin 禁用插件
func (s *Server) DisablePlugin(name string) error {
	if s.pluginRegistry == nil {
		return fmt.Errorf("plugin registry not initialized")
	}
	return s.pluginRegistry.Disable(name)
}

// InstallPluginsToClient 安装插件到客户端
func (s *Server) InstallPluginsToClient(clientID string, plugins []string) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	// 发送安装请求到客户端
	if err := s.sendInstallPlugins(cs.Session, plugins); err != nil {
		return err
	}

	// 更新数据库中客户端的已安装插件列表
	client, err := s.clientStore.GetClient(clientID)
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}

	// 获取插件版本信息并添加到客户端插件列表
	for _, pluginName := range plugins {
		// 检查是否已安装
		found := false
		for _, cp := range client.Plugins {
			if cp.Name == pluginName {
				found = true
				break
			}
		}
		if !found {
			client.Plugins = append(client.Plugins, db.ClientPlugin{
				Name:    pluginName,
				Version: "1.0.0",
				Enabled: true,
			})
		}
	}

	return s.clientStore.UpdateClient(client)
}

// sendInstallPlugins 发送安装插件请求
func (s *Server) sendInstallPlugins(session *yamux.Session, plugins []string) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.InstallPluginsRequest{Plugins: plugins}
	msg, err := protocol.NewMessage(protocol.MsgTypeInstallPlugins, req)
	if err != nil {
		return err
	}
	return protocol.WriteMessage(stream, msg)
}

// startUDPListener 启动 UDP 监听
func (s *Server) startUDPListener(cs *ClientSession, rule protocol.ProxyRule) {
	if err := s.portManager.Reserve(rule.RemotePort, cs.ID); err != nil {
		log.Printf("[Server] UDP port %d error: %v", rule.RemotePort, err)
		return
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", rule.RemotePort))
	if err != nil {
		log.Printf("[Server] UDP resolve error: %v", err)
		s.portManager.Release(rule.RemotePort)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("[Server] UDP listen %d error: %v", rule.RemotePort, err)
		s.portManager.Release(rule.RemotePort)
		return
	}

	cs.mu.Lock()
	cs.UDPConns[rule.RemotePort] = conn
	cs.mu.Unlock()

	log.Printf("[Server] UDP proxy %s: :%d -> %s:%d",
		rule.Name, rule.RemotePort, rule.LocalIP, rule.LocalPort)

	go s.handleUDPConn(cs, conn, rule)
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
	}
}

// GetPluginConfigSchema 获取插件配置模式
func (s *Server) GetPluginConfigSchema(name string) ([]router.ConfigField, error) {
	if s.pluginRegistry == nil {
		return nil, fmt.Errorf("plugin registry not initialized")
	}

	handler, err := s.pluginRegistry.GetClient(name)
	if err != nil {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	metadata := handler.Metadata()
	var result []router.ConfigField
	for _, f := range metadata.ConfigSchema {
		result = append(result, router.ConfigField{
			Key:         f.Key,
			Label:       f.Label,
			Type:        string(f.Type),
			Default:     f.Default,
			Required:    f.Required,
			Options:     f.Options,
			Description: f.Description,
		})
	}
	return result, nil
}

// SyncPluginConfigToClient 同步插件配置到客户端
func (s *Server) SyncPluginConfigToClient(clientID string, pluginName string, config map[string]string) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not online", clientID)
	}

	return s.sendPluginConfig(cs.Session, pluginName, config)
}

// sendPluginConfig 发送插件配置到客户端
func (s *Server) sendPluginConfig(session *yamux.Session, pluginName string, config map[string]string) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.PluginConfigSync{
		PluginName: pluginName,
		Config:     config,
	}
	msg, err := protocol.NewMessage(protocol.MsgTypePluginConfig, req)
	if err != nil {
		return err
	}
	return protocol.WriteMessage(stream, msg)
}

// InstallJSPluginToClient 安装 JS 插件到客户端
func (s *Server) InstallJSPluginToClient(clientID string, req router.JSPluginInstallRequest) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not online", clientID)
	}

	stream, err := cs.Session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	installReq := protocol.JSPluginInstallRequest{
		PluginID:   req.PluginID,
		PluginName: req.PluginName,
		Source:     req.Source,
		Signature:  req.Signature,
		RuleName:   req.RuleName,
		RemotePort: req.RemotePort,
		Config:     req.Config,
		AutoStart:  req.AutoStart,
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeJSPluginInstall, installReq)
	if err != nil {
		return err
	}

	if err := protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	// 等待安装结果
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return err
	}

	var result protocol.JSPluginInstallResult
	if err := resp.ParsePayload(&result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("install failed: %s", result.Error)
	}

	log.Printf("[Server] JS plugin %s installed on client %s", req.PluginName, clientID)
	return nil
}

// isClientPlugin 检查是否为客户端插件
func (s *Server) isClientPlugin(pluginType string) bool {
	if s.pluginRegistry == nil {
		return false
	}
	handler, err := s.pluginRegistry.GetClient(pluginType)
	if err != nil {
		return false
	}
	return handler != nil
}

// startClientPluginListener 启动客户端插件监听
func (s *Server) startClientPluginListener(cs *ClientSession, rule protocol.ProxyRule) {
	if err := s.portManager.Reserve(rule.RemotePort, cs.ID); err != nil {
		log.Printf("[Server] Port %d error: %v", rule.RemotePort, err)
		return
	}

	// 发送启动命令到客户端
	if err := s.sendClientPluginStart(cs.Session, rule); err != nil {
		log.Printf("[Server] Failed to start client plugin %s: %v", rule.Type, err)
		s.portManager.Release(rule.RemotePort)
		return
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", rule.RemotePort))
	if err != nil {
		log.Printf("[Server] Listen %d error: %v", rule.RemotePort, err)
		s.portManager.Release(rule.RemotePort)
		return
	}

	cs.mu.Lock()
	cs.Listeners[rule.RemotePort] = ln
	cs.mu.Unlock()

	log.Printf("[Server] Client plugin %s on :%d", rule.Type, rule.RemotePort)
	go s.acceptClientPluginConns(cs, ln, rule)
}

// sendClientPluginStart 发送客户端插件启动命令
func (s *Server) sendClientPluginStart(session *yamux.Session, rule protocol.ProxyRule) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.ClientPluginStartRequest{
		PluginName: rule.Type,
		RuleName:   rule.Name,
		RemotePort: rule.RemotePort,
		Config:     rule.PluginConfig,
	}
	msg, err := protocol.NewMessage(protocol.MsgTypeClientPluginStart, req)
	if err != nil {
		return err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	// 等待响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return err
	}
	if resp.Type != protocol.MsgTypeClientPluginStatus {
		return fmt.Errorf("unexpected response type: %d", resp.Type)
	}

	var status protocol.ClientPluginStatusResponse
	if err := resp.ParsePayload(&status); err != nil {
		return err
	}
	if !status.Running {
		return fmt.Errorf("plugin failed: %s", status.Error)
	}
	return nil
}

// acceptClientPluginConns 接受客户端插件连接
func (s *Server) acceptClientPluginConns(cs *ClientSession, ln net.Listener, rule protocol.ProxyRule) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handleClientPluginConn(cs, conn, rule)
	}
}

// handleClientPluginConn 处理客户端插件连接
func (s *Server) handleClientPluginConn(cs *ClientSession, conn net.Conn, rule protocol.ProxyRule) {
	defer conn.Close()

	// 如果启用了 HTTP Basic Auth，先进行认证
	var bufferedData []byte
	if rule.AuthEnabled {
		authenticated, data := s.checkHTTPBasicAuth(conn, rule.AuthUsername, rule.AuthPassword)
		if !authenticated {
			return
		}
		bufferedData = data
	}

	stream, err := cs.Session.Open()
	if err != nil {
		log.Printf("[Server] Open stream error: %v", err)
		return
	}
	defer stream.Close()

	req := protocol.ClientPluginConnRequest{
		PluginName: rule.Type,
		RuleName:   rule.Name,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeClientPluginConn, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return
	}

	// 如果有缓冲的数据（已读取的 HTTP 请求头），先发送给客户端
	if len(bufferedData) > 0 {
		if _, err := stream.Write(bufferedData); err != nil {
			return
		}
	}

	relay.Relay(conn, stream)
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

// autoPushJSPlugins 自动推送 JS 插件到客户端
func (s *Server) autoPushJSPlugins(cs *ClientSession) {
	// 记录已推送的插件，避免重复推送
	pushedPlugins := make(map[string]bool)

	// 1. 推送配置文件中的 JS 插件
	for _, jp := range s.jsPlugins {
		if !s.shouldPushToClient(jp.AutoPush, cs.ID) {
			continue
		}

		log.Printf("[Server] Auto-pushing JS plugin %s to client %s", jp.Name, cs.ID)

		req := router.JSPluginInstallRequest{
			PluginName: jp.Name,
			Source:     jp.Source,
			Signature:  jp.Signature,
			RuleName:   jp.Name,
			Config:     jp.Config,
			AutoStart:  jp.AutoStart,
		}

		if err := s.InstallJSPluginToClient(cs.ID, req); err != nil {
			log.Printf("[Server] Failed to push JS plugin %s: %v", jp.Name, err)
		} else {
			pushedPlugins[jp.Name] = true
		}
	}

	// 2. 推送客户端已安装的插件（从数据库）
	s.pushClientInstalledPlugins(cs, pushedPlugins)
}

// pushClientInstalledPlugins 推送客户端已安装的插件
func (s *Server) pushClientInstalledPlugins(cs *ClientSession, alreadyPushed map[string]bool) {
	if s.jsPluginStore == nil {
		return
	}

	// 获取客户端信息
	client, err := s.clientStore.GetClient(cs.ID)
	if err != nil {
		return
	}

	// 遍历客户端已安装的插件
	for _, cp := range client.Plugins {
		if !cp.Enabled {
			continue
		}

		// 跳过已推送的
		if alreadyPushed[cp.Name] {
			continue
		}

		// 从 JSPluginStore 获取插件完整信息
		jsPlugin, err := s.jsPluginStore.GetJSPlugin(cp.Name)
		if err != nil {
			log.Printf("[Server] JS plugin %s not found in store: %v", cp.Name, err)
			continue
		}

		log.Printf("[Server] Restoring installed plugin %s to client %s", cp.Name, cs.ID)

		// 合并配置（客户端配置优先）
		config := jsPlugin.Config
		if config == nil {
			config = make(map[string]string)
		}
		for k, v := range cp.Config {
			config[k] = v
		}

		req := router.JSPluginInstallRequest{
			PluginID:   cp.ID,
			PluginName: cp.Name,
			Source:     jsPlugin.Source,
			Signature:  jsPlugin.Signature,
			RuleName:   cp.Name,
			Config:     config,
			AutoStart:  jsPlugin.AutoStart,
		}

		if err := s.InstallJSPluginToClient(cs.ID, req); err != nil {
			log.Printf("[Server] Failed to restore plugin %s: %v", cp.Name, err)
		} else if cp.RemotePort > 0 {
			// 检查端口是否已在监听（避免重复启动）
			cs.mu.Lock()
			_, exists := cs.Listeners[cp.RemotePort]
			cs.mu.Unlock()
			if exists {
				continue
			}

			// 安装成功后启动服务端监听器
			pluginRule := protocol.ProxyRule{
				Name:         cp.Name,
				Type:         cp.Name,
				RemotePort:   cp.RemotePort,
				Enabled:      boolPtr(true),
				AuthEnabled:  cp.AuthEnabled,
				AuthUsername: cp.AuthUsername,
				AuthPassword: cp.AuthPassword,
			}
			s.startClientPluginListener(cs, pluginRule)
		}
	}
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

// StartClientPlugin 启动客户端插件
func (s *Server) StartClientPlugin(clientID, pluginName, ruleName string) error {
	s.mu.RLock()
	_, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	// 重新发送安装请求来启动插件
	return s.reinstallJSPlugin(clientID, pluginName, ruleName)
}

// StopClientPlugin 停止客户端插件
func (s *Server) StopClientPlugin(clientID, pluginName, ruleName string) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	return s.sendClientPluginStop(cs.Session, pluginName, ruleName)
}

// sendClientPluginStop 发送客户端插件停止命令
func (s *Server) sendClientPluginStop(session *yamux.Session, pluginName, ruleName string) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.ClientPluginStopRequest{
		PluginName: pluginName,
		RuleName:   ruleName,
	}
	msg, err := protocol.NewMessage(protocol.MsgTypeClientPluginStop, req)
	if err != nil {
		return err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	// 等待响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return err
	}
	if resp.Type != protocol.MsgTypeClientPluginStatus {
		return fmt.Errorf("unexpected response type: %d", resp.Type)
	}

	var status protocol.ClientPluginStatusResponse
	if err := resp.ParsePayload(&status); err != nil {
		return err
	}
	if status.Running {
		return fmt.Errorf("plugin still running: %s", status.Error)
	}
	return nil
}

// StartPluginRule 为客户端插件启动服务端监听器
func (s *Server) StartPluginRule(clientID string, rule protocol.ProxyRule) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	// 检查端口是否已被占用
	cs.mu.Lock()
	_, exists := cs.Listeners[rule.RemotePort]
	cs.mu.Unlock()
	if exists {
		// 端口已在监听，无需重复启动
		return nil
	}

	// 启动插件监听器
	s.startClientPluginListener(cs, rule)
	return nil
}

// StopPluginRule 停止客户端插件的服务端监听器
func (s *Server) StopPluginRule(clientID string, remotePort int) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return nil // 客户端不在线，无需停止
	}

	cs.mu.Lock()
	if ln, exists := cs.Listeners[remotePort]; exists {
		ln.Close()
		delete(cs.Listeners, remotePort)
	}
	cs.mu.Unlock()

	s.portManager.Release(remotePort)
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

// ProxyPluginAPIRequest 代理插件 API 请求到客户端
func (s *Server) ProxyPluginAPIRequest(clientID string, req protocol.PluginAPIRequest) (*protocol.PluginAPIResponse, error) {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("client %s not found or not online", clientID)
	}

	stream, err := cs.Session.Open()
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()

	// 设置超时（30秒）
	stream.SetDeadline(time.Now().Add(30 * time.Second))

	// 发送 API 请求
	msg, err := protocol.NewMessage(protocol.MsgTypePluginAPIRequest, req)
	if err != nil {
		return nil, err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return nil, err
	}

	// 读取响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.Type != protocol.MsgTypePluginAPIResponse {
		return nil, fmt.Errorf("unexpected response type: %d", resp.Type)
	}

	var apiResp protocol.PluginAPIResponse
	if err := resp.ParsePayload(&apiResp); err != nil {
		return nil, err
	}

	return &apiResp, nil
}

// RestartClientPlugin 重启客户端 JS 插件
func (s *Server) RestartClientPlugin(clientID, pluginName, ruleName string) error {
	s.mu.RLock()
	_, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	// 重新发送完整的安装请求来重启 JS 插件
	return s.reinstallJSPlugin(clientID, pluginName, ruleName)
}

// reinstallJSPlugin 重新安装 JS 插件（用于重启）
func (s *Server) reinstallJSPlugin(clientID, pluginName, ruleName string) error {
	// 从数据库获取插件信息
	if s.jsPluginStore == nil {
		return fmt.Errorf("JS plugin store not configured")
	}

	jsPlugin, err := s.jsPluginStore.GetJSPlugin(pluginName)
	if err != nil {
		return fmt.Errorf("plugin %s not found: %w", pluginName, err)
	}

	// 获取客户端的插件配置
	client, err := s.clientStore.GetClient(clientID)
	if err != nil {
		return fmt.Errorf("client not found: %w", err)
	}

	// 合并配置
	config := jsPlugin.Config
	if config == nil {
		config = make(map[string]string)
	}
	for _, cp := range client.Plugins {
		if cp.Name == pluginName {
			for k, v := range cp.Config {
				config[k] = v
			}
			break
		}
	}

	log.Printf("[Server] Reinstalling JS plugin %s to client %s", pluginName, clientID)

	req := router.JSPluginInstallRequest{
		PluginName: pluginName,
		Source:     jsPlugin.Source,
		Signature:  jsPlugin.Signature,
		RuleName:   ruleName,
		Config:     config,
		AutoStart:  true, // 重启时总是自动启动
	}

	return s.InstallJSPluginToClient(clientID, req)
}

// sendJSPluginRestart 发送 JS 插件重启命令
func (s *Server) sendJSPluginRestart(session *yamux.Session, pluginName, ruleName string) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	// 使用 PluginConfigUpdate 消息触发重启
	req := protocol.PluginConfigUpdateRequest{
		PluginName: pluginName,
		RuleName:   ruleName,
		Config:     nil,
		Restart:    true,
	}
	msg, err := protocol.NewMessage(protocol.MsgTypePluginConfigUpdate, req)
	if err != nil {
		return err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	// 等待响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return err
	}

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := resp.ParsePayload(&result); err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("restart failed: %s", result.Error)
	}

	log.Printf("[Server] JS plugin %s restarted on client", pluginName)
	return nil
}

// UpdateClientPluginConfig 更新客户端插件配置
func (s *Server) UpdateClientPluginConfig(clientID, pluginName, ruleName string, config map[string]string, restart bool) error {
	s.mu.RLock()
	cs, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	// 发送配置更新消息
	stream, err := cs.Session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.PluginConfigUpdateRequest{
		PluginName: pluginName,
		RuleName:   ruleName,
		Config:     config,
		Restart:    restart,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypePluginConfigUpdate, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	// 等待响应
	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return err
	}

	var result protocol.PluginConfigUpdateResponse
	if err := resp.ParsePayload(&result); err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("config update failed: %s", result.Error)
	}

	return nil
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

// boolPtr 返回 bool 值的指针
func boolPtr(b bool) *bool {
	return &b
}
