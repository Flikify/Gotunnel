package tunnel

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
	"github.com/hashicorp/yamux"
)

// 客户端常量
const (
	dialTimeout      = 10 * time.Second
	localDialTimeout = 5 * time.Second
	udpTimeout       = 10 * time.Second
	reconnectDelay   = 5 * time.Second
	disconnectDelay  = 3 * time.Second
	udpBufferSize    = 65535
	idFileName       = ".gotunnel_id"
)

// Client 隧道客户端
type Client struct {
	ServerAddr     string
	Token          string
	ID             string
	TLSEnabled     bool
	TLSConfig      *tls.Config
	session        *yamux.Session
	rules          []protocol.ProxyRule
	mu             sync.RWMutex
	pluginRegistry *plugin.Registry
}

// NewClient 创建客户端
func NewClient(serverAddr, token, id string) *Client {
	// 如果未指定 ID，尝试从本地文件加载
	if id == "" {
		id = loadClientID()
	}
	return &Client{
		ServerAddr: serverAddr,
		Token:      token,
		ID:         id,
	}
}

// getIDFilePath 获取 ID 文件路径
func getIDFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return idFileName
	}
	return filepath.Join(home, idFileName)
}

// loadClientID 从本地文件加载客户端 ID
func loadClientID() string {
	data, err := os.ReadFile(getIDFilePath())
	if err != nil {
		return ""
	}
	return string(data)
}

// saveClientID 保存客户端 ID 到本地文件
func saveClientID(id string) {
	if err := os.WriteFile(getIDFilePath(), []byte(id), 0600); err != nil {
		log.Printf("[Client] Failed to save client ID: %v", err)
	}
}

// SetPluginRegistry 设置插件注册表
func (c *Client) SetPluginRegistry(registry *plugin.Registry) {
	c.pluginRegistry = registry
}

// Run 启动客户端（带断线重连）
func (c *Client) Run() error {
	for {
		if err := c.connect(); err != nil {
			log.Printf("[Client] Connect error: %v", err)
			log.Printf("[Client] Reconnecting in %v...", reconnectDelay)
			time.Sleep(reconnectDelay)
			continue
		}

		c.handleSession()
		log.Printf("[Client] Disconnected, reconnecting...")
		time.Sleep(disconnectDelay)
	}
}

// connect 连接到服务端并认证
func (c *Client) connect() error {
	var conn net.Conn
	var err error

	if c.TLSEnabled && c.TLSConfig != nil {
		dialer := &net.Dialer{Timeout: dialTimeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", c.ServerAddr, c.TLSConfig)
	} else {
		conn, err = net.DialTimeout("tcp", c.ServerAddr, dialTimeout)
	}
	if err != nil {
		return err
	}

	authReq := protocol.AuthRequest{ClientID: c.ID, Token: c.Token}
	msg, _ := protocol.NewMessage(protocol.MsgTypeAuth, authReq)
	if err := protocol.WriteMessage(conn, msg); err != nil {
		conn.Close()
		return err
	}

	resp, err := protocol.ReadMessage(conn)
	if err != nil {
		conn.Close()
		return err
	}

	var authResp protocol.AuthResponse
	if err := resp.ParsePayload(&authResp); err != nil {
		conn.Close()
		return fmt.Errorf("parse auth response: %w", err)
	}
	if !authResp.Success {
		conn.Close()
		return fmt.Errorf("auth failed: %s", authResp.Message)
	}

	// 如果服务端分配了新 ID，则更新并保存
	if authResp.ClientID != "" && authResp.ClientID != c.ID {
		c.ID = authResp.ClientID
		saveClientID(c.ID)
		log.Printf("[Client] New ID assigned and saved: %s", c.ID)
	}

	log.Printf("[Client] Authenticated as %s", c.ID)

	session, err := yamux.Client(conn, nil)
	if err != nil {
		conn.Close()
		return err
	}

	c.mu.Lock()
	c.session = session
	c.mu.Unlock()

	return nil
}

// handleSession 处理会话
func (c *Client) handleSession() {
	defer c.session.Close()

	for {
		stream, err := c.session.Accept()
		if err != nil {
			return
		}
		go c.handleStream(stream)
	}
}

// handleStream 处理流
func (c *Client) handleStream(stream net.Conn) {
	msg, err := protocol.ReadMessage(stream)
	if err != nil {
		stream.Close()
		return
	}

	switch msg.Type {
	case protocol.MsgTypeProxyConfig:
		defer stream.Close()
		c.handleProxyConfig(msg)
	case protocol.MsgTypeNewProxy:
		defer stream.Close()
		c.handleNewProxy(stream, msg)
	case protocol.MsgTypeHeartbeat:
		defer stream.Close()
		c.handleHeartbeat(stream)
	case protocol.MsgTypeProxyConnect:
		c.handleProxyConnect(stream, msg)
	case protocol.MsgTypeUDPData:
		c.handleUDPData(stream, msg)
	case protocol.MsgTypePluginConfig:
		defer stream.Close()
		c.handlePluginConfig(msg)
	}
}

// handleProxyConfig 处理代理配置
func (c *Client) handleProxyConfig(msg *protocol.Message) {
	var cfg protocol.ProxyConfig
	if err := msg.ParsePayload(&cfg); err != nil {
		log.Printf("[Client] Parse proxy config error: %v", err)
		return
	}

	c.mu.Lock()
	c.rules = cfg.Rules
	c.mu.Unlock()

	log.Printf("[Client] Received %d proxy rules", len(cfg.Rules))
	for _, r := range cfg.Rules {
		log.Printf("[Client]   %s: %s:%d", r.Name, r.LocalIP, r.LocalPort)
	}
}

// handleNewProxy 处理新代理请求
func (c *Client) handleNewProxy(stream net.Conn, msg *protocol.Message) {
	var req protocol.NewProxyRequest
	if err := msg.ParsePayload(&req); err != nil {
		log.Printf("[Client] Parse new proxy request error: %v", err)
		return
	}

	var rule *protocol.ProxyRule
	c.mu.RLock()
	for _, r := range c.rules {
		if r.RemotePort == req.RemotePort {
			rule = &r
			break
		}
	}
	c.mu.RUnlock()

	if rule == nil {
		log.Printf("[Client] Unknown port %d", req.RemotePort)
		return
	}

	localAddr := fmt.Sprintf("%s:%d", rule.LocalIP, rule.LocalPort)
	localConn, err := net.DialTimeout("tcp", localAddr, localDialTimeout)
	if err != nil {
		log.Printf("[Client] Connect %s error: %v", localAddr, err)
		return
	}

	relay.Relay(stream, localConn)
}

// handleHeartbeat 处理心跳
func (c *Client) handleHeartbeat(stream net.Conn) {
	msg := &protocol.Message{Type: protocol.MsgTypeHeartbeatAck}
	protocol.WriteMessage(stream, msg)
}

// handleProxyConnect 处理代理连接请求 (SOCKS5/HTTP)
func (c *Client) handleProxyConnect(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ProxyConnectRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.sendProxyResult(stream, false, "invalid request")
		return
	}

	// 连接目标地址
	targetConn, err := net.DialTimeout("tcp", req.Target, dialTimeout)
	if err != nil {
		c.sendProxyResult(stream, false, err.Error())
		return
	}
	defer targetConn.Close()

	// 发送成功响应
	if err := c.sendProxyResult(stream, true, ""); err != nil {
		return
	}

	// 双向转发数据
	relay.Relay(stream, targetConn)
}

// sendProxyResult 发送代理连接结果
func (c *Client) sendProxyResult(stream net.Conn, success bool, message string) error {
	result := protocol.ProxyConnectResult{Success: success, Message: message}
	msg, _ := protocol.NewMessage(protocol.MsgTypeProxyResult, result)
	return protocol.WriteMessage(stream, msg)
}

// handleUDPData 处理 UDP 数据
func (c *Client) handleUDPData(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var packet protocol.UDPPacket
	if err := msg.ParsePayload(&packet); err != nil {
		return
	}

	// 查找对应的规则
	rule := c.findRuleByPort(packet.RemotePort)
	if rule == nil {
		return
	}

	// 连接本地 UDP 服务
	target := fmt.Sprintf("%s:%d", rule.LocalIP, rule.LocalPort)
	conn, err := net.DialTimeout("udp", target, localDialTimeout)
	if err != nil {
		return
	}
	defer conn.Close()

	// 发送数据到本地服务
	conn.SetDeadline(time.Now().Add(udpTimeout))
	if _, err := conn.Write(packet.Data); err != nil {
		return
	}

	// 读取响应
	buf := make([]byte, udpBufferSize)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	// 发送响应回服务端
	respPacket := protocol.UDPPacket{
		RemotePort: packet.RemotePort,
		ClientAddr: packet.ClientAddr,
		Data:       buf[:n],
	}
	respMsg, _ := protocol.NewMessage(protocol.MsgTypeUDPData, respPacket)
	protocol.WriteMessage(stream, respMsg)
}

// findRuleByPort 根据端口查找规则
func (c *Client) findRuleByPort(port int) *protocol.ProxyRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := range c.rules {
		if c.rules[i].RemotePort == port {
			return &c.rules[i]
		}
	}
	return nil
}

// handlePluginConfig 处理插件配置同步
func (c *Client) handlePluginConfig(msg *protocol.Message) {
	var cfg protocol.PluginConfigSync
	if err := msg.ParsePayload(&cfg); err != nil {
		log.Printf("[Client] Parse plugin config error: %v", err)
		return
	}

	log.Printf("[Client] Received config for plugin: %s", cfg.PluginName)

	// 应用配置到插件
	if c.pluginRegistry != nil {
		handler, err := c.pluginRegistry.Get(cfg.PluginName)
		if err != nil {
			log.Printf("[Client] Plugin %s not found: %v", cfg.PluginName, err)
			return
		}
		if err := handler.Init(cfg.Config); err != nil {
			log.Printf("[Client] Plugin %s init error: %v", cfg.PluginName, err)
			return
		}
		log.Printf("[Client] Plugin %s config applied", cfg.PluginName)
	}
}
