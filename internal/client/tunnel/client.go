package tunnel

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
	"github.com/hashicorp/yamux"
)

// Client 隧道客户端
type Client struct {
	ServerAddr string
	Token      string
	ID         string
	TLSEnabled bool
	TLSConfig  *tls.Config
	session    *yamux.Session
	rules      []protocol.ProxyRule
	mu         sync.RWMutex
}

// NewClient 创建客户端
func NewClient(serverAddr, token, id string) *Client {
	if id == "" {
		id = uuid.New().String()[:8]
	}
	return &Client{
		ServerAddr: serverAddr,
		Token:      token,
		ID:         id,
	}
}

// Run 启动客户端（带断线重连）
func (c *Client) Run() error {
	for {
		if err := c.connect(); err != nil {
			log.Printf("[Client] Connect error: %v", err)
			log.Printf("[Client] Reconnecting in 5s...")
			time.Sleep(5 * time.Second)
			continue
		}

		c.handleSession()
		log.Printf("[Client] Disconnected, reconnecting...")
		time.Sleep(3 * time.Second)
	}
}

// connect 连接到服务端并认证
func (c *Client) connect() error {
	var conn net.Conn
	var err error

	if c.TLSEnabled && c.TLSConfig != nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, err = tls.DialWithDialer(dialer, "tcp", c.ServerAddr, c.TLSConfig)
	} else {
		conn, err = net.DialTimeout("tcp", c.ServerAddr, 10*time.Second)
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
	resp.ParsePayload(&authResp)
	if !authResp.Success {
		conn.Close()
		return fmt.Errorf("auth failed: %s", authResp.Message)
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
	}
}

// handleProxyConfig 处理代理配置
func (c *Client) handleProxyConfig(msg *protocol.Message) {
	var cfg protocol.ProxyConfig
	msg.ParsePayload(&cfg)

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
	msg.ParsePayload(&req)

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
	localConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
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
	targetConn, err := net.DialTimeout("tcp", req.Target, 10*time.Second)
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
