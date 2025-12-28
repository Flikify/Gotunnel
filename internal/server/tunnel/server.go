package tunnel

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router"
	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/proxy"
	"github.com/gotunnel/pkg/relay"
	"github.com/gotunnel/pkg/utils"
	"github.com/hashicorp/yamux"
)

// 服务端常量
const (
	authTimeout      = 10 * time.Second
	heartbeatTimeout = 10 * time.Second
	udpBufferSize    = 65535
)

// generateClientID 生成随机客户端 ID
func generateClientID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Server 隧道服务端
type Server struct {
	clientStore    db.ClientStore
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
}

// ClientSession 客户端会话
type ClientSession struct {
	ID          string
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
	}
}

// SetTLSConfig 设置 TLS 配置
func (s *Server) SetTLSConfig(config *tls.Config) {
	s.tlsConfig = config
}

// SetPluginRegistry 设置插件注册表
func (s *Server) SetPluginRegistry(registry *plugin.Registry) {
	s.pluginRegistry = registry
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
	defer ln.Close()

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
		s.sendAuthResponse(conn, false, "invalid token", "")
		return
	}

	// 处理客户端 ID
	clientID := authReq.ClientID
	if clientID == "" {
		clientID = generateClientID()
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

	log.Printf("[Server] Client %s authenticated", clientID)
	s.setupClientSession(conn, clientID, rules)
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
		UDPConns:  make(map[int]*net.UDPConn),
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

	// 优先使用插件系统
	if s.pluginRegistry != nil {
		if handler, err := s.pluginRegistry.Get(rule.Type); err == nil {
			handler.Init(rule.PluginConfig)
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				go handler.HandleConn(conn, dialer)
			}
		}
	}

	// 回退到内置 proxy 实现
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
	// 先复制客户端引用，避免嵌套锁
	s.mu.RLock()
	clients := make([]*ClientSession, 0, len(s.clients))
	for _, cs := range s.clients {
		clients = append(clients, cs)
	}
	s.mu.RUnlock()

	result := make(map[string]struct {
		Online   bool
		LastPing string
	})

	for _, cs := range clients {
		cs.mu.Lock()
		result[cs.ID] = struct {
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
			// 获取插件信息
			version := "1.0.0"
			if handler, err := s.pluginRegistry.Get(pluginName); err == nil && handler != nil {
				version = handler.Metadata().Version
			}
			client.Plugins = append(client.Plugins, db.ClientPlugin{
				Name:    pluginName,
				Version: version,
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

	handler, err := s.pluginRegistry.Get(name)
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
