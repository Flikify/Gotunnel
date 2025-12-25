package tunnel

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gotunnel/pkg/protocol"
	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

// Client 隧道客户端
type Client struct {
	ServerAddr string
	Token      string
	ID         string
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
	conn, err := net.DialTimeout("tcp", c.ServerAddr, 10*time.Second)
	if err != nil {
		return err
	}

	// 发送认证
	authReq := protocol.AuthRequest{ClientID: c.ID, Token: c.Token}
	msg, _ := protocol.NewMessage(protocol.MsgTypeAuth, authReq)
	if err := protocol.WriteMessage(conn, msg); err != nil {
		conn.Close()
		return err
	}

	// 读取响应
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

	// 建立 Yamux 会话
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
	defer stream.Close()

	msg, err := protocol.ReadMessage(stream)
	if err != nil {
		return
	}

	switch msg.Type {
	case protocol.MsgTypeProxyConfig:
		c.handleProxyConfig(msg)
	case protocol.MsgTypeNewProxy:
		c.handleNewProxy(stream, msg)
	case protocol.MsgTypeHeartbeat:
		c.handleHeartbeat(stream)
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

	// 查找对应规则
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

	// 连接本地服务
	localAddr := fmt.Sprintf("%s:%d", rule.LocalIP, rule.LocalPort)
	localConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
	if err != nil {
		log.Printf("[Client] Connect %s error: %v", localAddr, err)
		return
	}

	// 双向转发
	relay(stream, localConn)
}

// handleHeartbeat 处理心跳
func (c *Client) handleHeartbeat(stream net.Conn) {
	msg := &protocol.Message{Type: protocol.MsgTypeHeartbeatAck}
	protocol.WriteMessage(stream, msg)
}
