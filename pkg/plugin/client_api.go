package plugin

import (
	"context"
	"fmt"
	"net"
	"time"
)

// =============================================================================
// 客户端 API 实现
// =============================================================================

// ClientAPI 客户端 PluginAPI 实现
type ClientAPI struct {
	*baseAPI
	clientID string
	dialer   Dialer
}

// ClientAPIOption 客户端 API 配置选项
type ClientAPIOption struct {
	PluginName string
	ClientID   string
	Config     map[string]string
	Dialer     Dialer
}

// NewClientAPI 创建客户端 API
func NewClientAPI(opt ClientAPIOption) *ClientAPI {
	return &ClientAPI{
		baseAPI:  newBaseAPI(opt.PluginName, opt.Config),
		clientID: opt.ClientID,
		dialer:   opt.Dialer,
	}
}

// --- 网络操作 ---

// Dial 通过隧道建立连接
func (c *ClientAPI) Dial(network, address string) (net.Conn, error) {
	if c.dialer == nil {
		return nil, ErrNotConnected
	}
	return c.dialer.Dial(network, address)
}

// DialTimeout 带超时的连接（使用 context 避免 goroutine 泄漏）
func (c *ClientAPI) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if c.dialer == nil {
		return nil, ErrNotConnected
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		conn, err := c.dialer.Dial(network, address)
		select {
		case ch <- result{conn, err}:
		case <-ctx.Done():
			if conn != nil {
				conn.Close()
			}
		}
	}()

	select {
	case r := <-ch:
		return r.conn, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("dial timeout")
	}
}

// Listen 客户端不支持监听
func (c *ClientAPI) Listen(network, address string) (net.Listener, error) {
	return nil, ErrNotSupported
}

// --- 端口管理（客户端不支持）---

// ReservePort 客户端不支持
func (c *ClientAPI) ReservePort(port int) error {
	return ErrNotSupported
}

// ReleasePort 客户端不支持
func (c *ClientAPI) ReleasePort(port int) {}

// IsPortAvailable 检查本地端口是否可用
func (c *ClientAPI) IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// --- 规则管理（客户端不支持）---

// CreateRule 客户端不支持
func (c *ClientAPI) CreateRule(rule *RuleConfig) error {
	return ErrNotSupported
}

// DeleteRule 客户端不支持
func (c *ClientAPI) DeleteRule(clientID, ruleName string) error {
	return ErrNotSupported
}

// GetRules 客户端不支持
func (c *ClientAPI) GetRules(clientID string) ([]RuleConfig, error) {
	return nil, ErrNotSupported
}

// UpdateRule 客户端不支持
func (c *ClientAPI) UpdateRule(clientID string, rule *RuleConfig) error {
	return ErrNotSupported
}

// --- 客户端管理 ---

// GetClientID 获取当前客户端 ID
func (c *ClientAPI) GetClientID() string {
	return c.clientID
}

// GetClientList 客户端不支持
func (c *ClientAPI) GetClientList() ([]ClientInfo, error) {
	return nil, ErrNotSupported
}

// IsClientOnline 客户端不支持
func (c *ClientAPI) IsClientOnline(clientID string) bool {
	return false
}

// --- 上下文 ---

// GetContext 获取当前上下文
func (c *ClientAPI) GetContext() *Context {
	return &Context{
		PluginName: c.getPluginName(),
		Side:       SideClient,
		ClientID:   c.clientID,
		Config:     c.getConfigMap(),
	}
}

// GetServerInfo 客户端不支持
func (c *ClientAPI) GetServerInfo() *ServerInfo {
	return nil
}
