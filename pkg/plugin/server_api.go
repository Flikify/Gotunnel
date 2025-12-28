package plugin

import (
	"net"
	"time"
)

// =============================================================================
// 服务端依赖接口（依赖注入）
// =============================================================================

// PortStore 端口存储接口
type PortStore interface {
	Reserve(port int, owner string) error
	Release(port int)
	IsAvailable(port int) bool
}

// RuleStore 规则存储接口
type RuleStore interface {
	GetAll(clientID string) ([]RuleConfig, error)
	Create(clientID string, rule *RuleConfig) error
	Update(clientID string, rule *RuleConfig) error
	Delete(clientID, ruleName string) error
}

// ClientStore 客户端存储接口
type ClientStore interface {
	GetAll() ([]ClientInfo, error)
	IsOnline(clientID string) bool
}

// =============================================================================
// 服务端 API 实现
// =============================================================================

// ServerAPI 服务端 PluginAPI 实现
type ServerAPI struct {
	*baseAPI
	portStore   PortStore
	ruleStore   RuleStore
	clientStore ClientStore
	serverInfo  *ServerInfo
}

// ServerAPIOption 服务端 API 配置选项
type ServerAPIOption struct {
	PluginName  string
	Config      map[string]string
	PortStore   PortStore
	RuleStore   RuleStore
	ClientStore ClientStore
	ServerInfo  *ServerInfo
}

// NewServerAPI 创建服务端 API
func NewServerAPI(opt ServerAPIOption) *ServerAPI {
	return &ServerAPI{
		baseAPI:     newBaseAPI(opt.PluginName, opt.Config),
		portStore:   opt.PortStore,
		ruleStore:   opt.RuleStore,
		clientStore: opt.ClientStore,
		serverInfo:  opt.ServerInfo,
	}
}

// --- 网络操作 ---

// Dial 服务端不支持隧道拨号
func (s *ServerAPI) Dial(network, address string) (net.Conn, error) {
	return nil, ErrNotSupported
}

// DialTimeout 服务端不支持隧道拨号
func (s *ServerAPI) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return nil, ErrNotSupported
}

// Listen 在指定地址监听
func (s *ServerAPI) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

// --- 端口管理 ---

// ReservePort 预留端口
func (s *ServerAPI) ReservePort(port int) error {
	if s.portStore == nil {
		return ErrNotSupported
	}
	return s.portStore.Reserve(port, s.getPluginName())
}

// ReleasePort 释放端口
func (s *ServerAPI) ReleasePort(port int) {
	if s.portStore != nil {
		s.portStore.Release(port)
	}
}

// IsPortAvailable 检查端口是否可用
func (s *ServerAPI) IsPortAvailable(port int) bool {
	if s.portStore == nil {
		return false
	}
	return s.portStore.IsAvailable(port)
}

// --- 规则管理 ---

// CreateRule 创建代理规则
func (s *ServerAPI) CreateRule(rule *RuleConfig) error {
	if s.ruleStore == nil {
		return ErrNotSupported
	}
	return s.ruleStore.Create(rule.ClientID, rule)
}

// DeleteRule 删除代理规则
func (s *ServerAPI) DeleteRule(clientID, ruleName string) error {
	if s.ruleStore == nil {
		return ErrNotSupported
	}
	return s.ruleStore.Delete(clientID, ruleName)
}

// GetRules 获取客户端的代理规则
func (s *ServerAPI) GetRules(clientID string) ([]RuleConfig, error) {
	if s.ruleStore == nil {
		return nil, ErrNotSupported
	}
	return s.ruleStore.GetAll(clientID)
}

// UpdateRule 更新代理规则
func (s *ServerAPI) UpdateRule(clientID string, rule *RuleConfig) error {
	if s.ruleStore == nil {
		return ErrNotSupported
	}
	return s.ruleStore.Update(clientID, rule)
}

// --- 客户端管理 ---

// GetClientID 服务端返回空
func (s *ServerAPI) GetClientID() string {
	return ""
}

// GetClientList 获取所有客户端列表
func (s *ServerAPI) GetClientList() ([]ClientInfo, error) {
	if s.clientStore == nil {
		return nil, ErrNotSupported
	}
	return s.clientStore.GetAll()
}

// IsClientOnline 检查客户端是否在线
func (s *ServerAPI) IsClientOnline(clientID string) bool {
	if s.clientStore == nil {
		return false
	}
	return s.clientStore.IsOnline(clientID)
}

// --- 上下文 ---

// GetContext 获取当前上下文
func (s *ServerAPI) GetContext() *Context {
	return &Context{
		PluginName: s.getPluginName(),
		Side:       SideServer,
		Config:     s.getConfigMap(),
	}
}

// GetServerInfo 获取服务端信息
func (s *ServerAPI) GetServerInfo() *ServerInfo {
	return s.serverInfo
}
