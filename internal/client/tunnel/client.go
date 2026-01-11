package tunnel

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/plugin/script"
	"github.com/gotunnel/pkg/plugin/sign"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
	"github.com/gotunnel/pkg/update"
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
	ServerAddr      string
	Token           string
	ID              string
	TLSEnabled      bool
	TLSConfig       *tls.Config
	DataDir         string // 数据目录
	session         *yamux.Session
	rules           []protocol.ProxyRule
	mu              sync.RWMutex
	pluginRegistry  *plugin.Registry
	runningPlugins  map[string]plugin.ClientPlugin
	versionStore    *PluginVersionStore
	pluginMu        sync.RWMutex
	logger          *Logger // 日志收集器
}

// NewClient 创建客户端
func NewClient(serverAddr, token, id string) *Client {
	if id == "" {
		id = loadClientID()
	}

	// 默认数据目录
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".gotunnel")

	// 初始化日志收集器
	logger, err := NewLogger(dataDir)
	if err != nil {
		log.Printf("[Client] Failed to initialize logger: %v", err)
	}

	return &Client{
		ServerAddr:     serverAddr,
		Token:          token,
		ID:             id,
		DataDir:        dataDir,
		runningPlugins: make(map[string]plugin.ClientPlugin),
		logger:         logger,
	}
}

// InitVersionStore 初始化版本存储
func (c *Client) InitVersionStore() error {
	store, err := NewPluginVersionStore(c.DataDir)
	if err != nil {
		return err
	}
	c.versionStore = store
	return nil
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

// logf 安全地记录日志（同时输出到标准日志和日志收集器）
func (c *Client) logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if c.logger != nil {
		c.logger.Printf(msg)
	}
}

// logErrorf 安全地记录错误日志
func (c *Client) logErrorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if c.logger != nil {
		c.logger.Errorf(msg)
	}
}

// logWarnf 安全地记录警告日志
func (c *Client) logWarnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if c.logger != nil {
		c.logger.Warnf(msg)
	}
}

// Run 启动客户端（带断线重连）
func (c *Client) Run() error {
	for {
		if err := c.connect(); err != nil {
			c.logErrorf("[Client] Connect error: %v", err)
			c.logf("[Client] Reconnecting in %v...", reconnectDelay)
			time.Sleep(reconnectDelay)
			continue
		}

		c.handleSession()
		c.logWarnf("[Client] Disconnected, reconnecting...")
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
		c.logf("[Client] New ID assigned and saved: %s", c.ID)
	}

	c.logf("[Client] Authenticated as %s", c.ID)

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
	case protocol.MsgTypeClientPluginStart:
		c.handleClientPluginStart(stream, msg)
	case protocol.MsgTypeClientPluginStop:
		c.handleClientPluginStop(stream, msg)
	case protocol.MsgTypeClientPluginConn:
		c.handleClientPluginConn(stream, msg)
	case protocol.MsgTypeJSPluginInstall:
		c.handleJSPluginInstall(stream, msg)
	case protocol.MsgTypeClientRestart:
		c.handleClientRestart(stream, msg)
	case protocol.MsgTypePluginConfigUpdate:
		c.handlePluginConfigUpdate(stream, msg)
	case protocol.MsgTypeUpdateDownload:
		c.handleUpdateDownload(stream, msg)
	case protocol.MsgTypeLogRequest:
		go c.handleLogRequest(stream, msg)
	case protocol.MsgTypeLogStop:
		c.handleLogStop(stream, msg)
	case protocol.MsgTypePluginStatusQuery:
		c.handlePluginStatusQuery(stream, msg)
	case protocol.MsgTypePluginAPIRequest:
		c.handlePluginAPIRequest(stream, msg)
	}
}

// handleProxyConfig 处理代理配置
func (c *Client) handleProxyConfig(msg *protocol.Message) {
	var cfg protocol.ProxyConfig
	if err := msg.ParsePayload(&cfg); err != nil {
		c.logErrorf("[Client] Parse proxy config error: %v", err)
		return
	}

	c.mu.Lock()
	c.rules = cfg.Rules
	c.mu.Unlock()

	c.logf("[Client] Received %d proxy rules", len(cfg.Rules))
	for _, r := range cfg.Rules {
		c.logf("[Client]   %s: %s:%d", r.Name, r.LocalIP, r.LocalPort)
	}
}

// handleNewProxy 处理新代理请求
func (c *Client) handleNewProxy(stream net.Conn, msg *protocol.Message) {
	var req protocol.NewProxyRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.logErrorf("[Client] Parse new proxy request error: %v", err)
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
		c.logWarnf("[Client] Unknown port %d", req.RemotePort)
		return
	}

	localAddr := fmt.Sprintf("%s:%d", rule.LocalIP, rule.LocalPort)
	localConn, err := net.DialTimeout("tcp", localAddr, localDialTimeout)
	if err != nil {
		c.logErrorf("[Client] Connect %s error: %v", localAddr, err)
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
		c.logErrorf("[Client] Parse plugin config error: %v", err)
		return
	}

	c.logf("[Client] Received config for plugin: %s", cfg.PluginName)

	// 应用配置到插件
	if c.pluginRegistry != nil {
		handler, err := c.pluginRegistry.GetClient(cfg.PluginName)
		if err != nil {
			c.logWarnf("[Client] Plugin %s not found: %v", cfg.PluginName, err)
			return
		}
		if err := handler.Init(cfg.Config); err != nil {
			c.logErrorf("[Client] Plugin %s init error: %v", cfg.PluginName, err)
			return
		}
		c.logf("[Client] Plugin %s config applied", cfg.PluginName)
	}
}

// handleClientPluginStart 处理客户端插件启动请求
func (c *Client) handleClientPluginStart(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ClientPluginStartRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.sendPluginStatus(stream, req.PluginName, req.RuleName, false, "", err.Error())
		return
	}

	c.logf("[Client] Starting plugin %s for rule %s", req.PluginName, req.RuleName)

	// 获取插件
	if c.pluginRegistry == nil {
		c.sendPluginStatus(stream, req.PluginName, req.RuleName, false, "", "plugin registry not set")
		return
	}

	handler, err := c.pluginRegistry.GetClient(req.PluginName)
	if err != nil {
		c.sendPluginStatus(stream, req.PluginName, req.RuleName, false, "", err.Error())
		return
	}

	// 初始化并启动
	if err := handler.Init(req.Config); err != nil {
		c.sendPluginStatus(stream, req.PluginName, req.RuleName, false, "", err.Error())
		return
	}

	localAddr, err := handler.Start()
	if err != nil {
		c.sendPluginStatus(stream, req.PluginName, req.RuleName, false, "", err.Error())
		return
	}

	// 保存运行中的插件
	key := req.PluginName + ":" + req.RuleName
	c.pluginMu.Lock()
	c.runningPlugins[key] = handler
	c.pluginMu.Unlock()

	c.logf("[Client] Plugin %s started at %s", req.PluginName, localAddr)
	c.sendPluginStatus(stream, req.PluginName, req.RuleName, true, localAddr, "")
}

// sendPluginStatus 发送插件状态响应
func (c *Client) sendPluginStatus(stream net.Conn, pluginName, ruleName string, running bool, localAddr, errMsg string) {
	resp := protocol.ClientPluginStatusResponse{
		PluginName: pluginName,
		RuleName:   ruleName,
		Running:    running,
		LocalAddr:  localAddr,
		Error:      errMsg,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeClientPluginStatus, resp)
	protocol.WriteMessage(stream, msg)
}

// handleClientPluginConn 处理客户端插件连接
func (c *Client) handleClientPluginConn(stream net.Conn, msg *protocol.Message) {
	var req protocol.ClientPluginConnRequest
	if err := msg.ParsePayload(&req); err != nil {
		stream.Close()
		return
	}

	c.pluginMu.RLock()
	var handler plugin.ClientPlugin
	var ok bool

	// 优先使用 PluginID 查找
	if req.PluginID != "" {
		handler, ok = c.runningPlugins[req.PluginID]
	}

	// 如果没找到，回退到 pluginName:ruleName
	if !ok {
		key := req.PluginName + ":" + req.RuleName
		handler, ok = c.runningPlugins[key]
	}
	c.pluginMu.RUnlock()

	if !ok {
		c.logWarnf("[Client] Plugin %s (ID: %s) not running", req.PluginName, req.PluginID)
		stream.Close()
		return
	}

	// 让插件处理连接
	handler.HandleConn(stream)
}

// handleJSPluginInstall 处理 JS 插件安装请求
func (c *Client) handleJSPluginInstall(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.JSPluginInstallRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.sendJSPluginResult(stream, "", false, err.Error())
		return
	}

	c.logf("[Client] Installing JS plugin: %s (ID: %s)", req.PluginName, req.PluginID)

	// 使用 PluginID 作为 key（如果有），否则回退到 pluginName:ruleName
	key := req.PluginID
	if key == "" {
		key = req.PluginName + ":" + req.RuleName
	}

	// 如果插件已经在运行，先停止它
	c.pluginMu.Lock()
	if existingHandler, ok := c.runningPlugins[key]; ok {
		c.logf("[Client] Stopping existing plugin %s before reinstall", key)
		if err := existingHandler.Stop(); err != nil {
			c.logErrorf("[Client] Stop existing plugin error: %v", err)
		}
		delete(c.runningPlugins, key)
	}
	c.pluginMu.Unlock()

	// 验证官方签名
	if err := c.verifyJSPluginSignature(req.PluginName, req.Source, req.Signature); err != nil {
		c.logErrorf("[Client] JS plugin %s signature verification failed: %v", req.PluginName, err)
		c.sendJSPluginResult(stream, req.PluginName, false, "signature verification failed: "+err.Error())
		return
	}
	c.logf("[Client] JS plugin %s signature verified", req.PluginName)

	// 创建 JS 插件
	jsPlugin, err := script.NewJSPlugin(req.PluginName, req.Source)
	if err != nil {
		c.sendJSPluginResult(stream, req.PluginName, false, err.Error())
		return
	}

	// 注册到 registry
	if c.pluginRegistry != nil {
		c.pluginRegistry.RegisterClient(jsPlugin)
	}

	c.logf("[Client] JS plugin %s installed", req.PluginName)

	// 保存版本信息（防止降级攻击）
	if c.versionStore != nil {
		signed, _ := sign.DecodeSignedPlugin(req.Signature)
		if signed != nil {
			c.versionStore.SetVersion(req.PluginName, signed.Payload.Version)
		}
	}

	// 先启动插件，再发送安装结果
	// 这样服务端收到结果后启动监听器时，客户端插件已经准备好了
	if req.AutoStart {
		c.startJSPlugin(jsPlugin, req)
	}

	c.sendJSPluginResult(stream, req.PluginName, true, "")
}

// sendJSPluginResult 发送 JS 插件安装结果
func (c *Client) sendJSPluginResult(stream net.Conn, name string, success bool, errMsg string) {
	result := protocol.JSPluginInstallResult{
		PluginName: name,
		Success:    success,
		Error:      errMsg,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeJSPluginResult, result)
	protocol.WriteMessage(stream, msg)
}

// startJSPlugin 启动 JS 插件
func (c *Client) startJSPlugin(handler plugin.ClientPlugin, req protocol.JSPluginInstallRequest) {
	if err := handler.Init(req.Config); err != nil {
		c.logErrorf("[Client] JS plugin %s init error: %v", req.PluginName, err)
		return
	}

	localAddr, err := handler.Start()
	if err != nil {
		c.logErrorf("[Client] JS plugin %s start error: %v", req.PluginName, err)
		return
	}

	// 使用 PluginID 作为 key（如果有），否则回退到 pluginName:ruleName
	key := req.PluginID
	if key == "" {
		key = req.PluginName + ":" + req.RuleName
	}
	c.pluginMu.Lock()
	c.runningPlugins[key] = handler
	c.pluginMu.Unlock()

	c.logf("[Client] JS plugin %s (ID: %s) started at %s", req.PluginName, req.PluginID, localAddr)
}

// verifyJSPluginSignature 验证 JS 插件签名
func (c *Client) verifyJSPluginSignature(pluginName, source, signature string) error {
	if signature == "" {
		return fmt.Errorf("missing signature")
	}

	// 解码签名
	signed, err := sign.DecodeSignedPlugin(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	// 根据 KeyID 获取对应公钥
	pubKey, err := sign.GetPublicKeyByID(signed.Payload.KeyID)
	if err != nil {
		return fmt.Errorf("get public key: %w", err)
	}

	// 验证插件名称匹配
	if signed.Payload.Name != pluginName {
		return fmt.Errorf("plugin name mismatch: expected %s, got %s",
			pluginName, signed.Payload.Name)
	}

	// 验证签名和源码哈希
	if err := sign.VerifyPlugin(pubKey, signed, source); err != nil {
		return err
	}

	// 检查版本降级攻击
	if c.versionStore != nil {
		currentVer := c.versionStore.GetVersion(pluginName)
		if currentVer != "" {
			cmp := sign.CompareVersions(signed.Payload.Version, currentVer)
			if cmp < 0 {
				return fmt.Errorf("version downgrade rejected: %s < %s",
					signed.Payload.Version, currentVer)
			}
		}
	}

	return nil
}

// handleClientPluginStop 处理客户端插件停止请求
func (c *Client) handleClientPluginStop(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ClientPluginStopRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.sendPluginStatus(stream, req.PluginName, req.RuleName, true, "", err.Error())
		return
	}

	c.pluginMu.Lock()
	var handler plugin.ClientPlugin
	var key string
	var ok bool

	// 优先使用 PluginID 查找
	if req.PluginID != "" {
		handler, ok = c.runningPlugins[req.PluginID]
		if ok {
			key = req.PluginID
		}
	}

	// 如果没找到，回退到 pluginName:ruleName
	if !ok {
		key = req.PluginName + ":" + req.RuleName
		handler, ok = c.runningPlugins[key]
	}

	if ok {
		if err := handler.Stop(); err != nil {
			c.logErrorf("[Client] Plugin %s stop error: %v", key, err)
		}
		delete(c.runningPlugins, key)
	}
	c.pluginMu.Unlock()

	c.logf("[Client] Plugin %s stopped", key)
	c.sendPluginStatus(stream, req.PluginName, req.RuleName, false, "", "")
}

// handleClientRestart 处理客户端重启请求
func (c *Client) handleClientRestart(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ClientRestartRequest
	msg.ParsePayload(&req)

	c.logf("[Client] Restart requested: %s", req.Reason)

	// 发送响应
	resp := protocol.ClientRestartResponse{
		Success: true,
		Message: "restarting",
	}
	respMsg, _ := protocol.NewMessage(protocol.MsgTypeClientRestart, resp)
	protocol.WriteMessage(stream, respMsg)

	// 停止所有运行中的插件
	c.pluginMu.Lock()
	for key, handler := range c.runningPlugins {
		c.logf("[Client] Stopping plugin %s for restart", key)
		handler.Stop()
	}
	c.runningPlugins = make(map[string]plugin.ClientPlugin)
	c.pluginMu.Unlock()

	// 关闭会话（会触发重连）
	if c.session != nil {
		c.session.Close()
	}
}

// handlePluginConfigUpdate 处理插件配置更新请求
func (c *Client) handlePluginConfigUpdate(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.PluginConfigUpdateRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.sendPluginConfigUpdateResult(stream, req.PluginName, req.RuleName, false, err.Error())
		return
	}

	c.pluginMu.RLock()
	var handler plugin.ClientPlugin
	var key string
	var ok bool

	// 优先使用 PluginID 查找
	if req.PluginID != "" {
		handler, ok = c.runningPlugins[req.PluginID]
		if ok {
			key = req.PluginID
		}
	}

	// 如果没找到，回退到 pluginName:ruleName
	if !ok {
		key = req.PluginName + ":" + req.RuleName
		handler, ok = c.runningPlugins[key]
	}
	c.pluginMu.RUnlock()

	c.logf("[Client] Config update for plugin %s", key)

	if !ok {
		c.sendPluginConfigUpdateResult(stream, req.PluginName, req.RuleName, false, "plugin not running")
		return
	}

	if req.Restart {
		// 停止并重启插件
		c.pluginMu.Lock()
		if err := handler.Stop(); err != nil {
			c.logErrorf("[Client] Plugin %s stop error: %v", key, err)
		}
		delete(c.runningPlugins, key)
		c.pluginMu.Unlock()

		// 重新初始化和启动
		if err := handler.Init(req.Config); err != nil {
			c.sendPluginConfigUpdateResult(stream, req.PluginName, req.RuleName, false, err.Error())
			return
		}

		localAddr, err := handler.Start()
		if err != nil {
			c.sendPluginConfigUpdateResult(stream, req.PluginName, req.RuleName, false, err.Error())
			return
		}

		c.pluginMu.Lock()
		c.runningPlugins[key] = handler
		c.pluginMu.Unlock()

		c.logf("[Client] Plugin %s restarted at %s with new config", key, localAddr)
	}

	c.sendPluginConfigUpdateResult(stream, req.PluginName, req.RuleName, true, "")
}

// sendPluginConfigUpdateResult 发送插件配置更新结果
func (c *Client) sendPluginConfigUpdateResult(stream net.Conn, pluginName, ruleName string, success bool, errMsg string) {
	result := protocol.PluginConfigUpdateResponse{
		PluginName: pluginName,
		RuleName:   ruleName,
		Success:    success,
		Error:      errMsg,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypePluginConfigUpdate, result)
	protocol.WriteMessage(stream, msg)
}

// handleUpdateDownload 处理更新下载请求
func (c *Client) handleUpdateDownload(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.UpdateDownloadRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.logErrorf("[Client] Parse update request error: %v", err)
		c.sendUpdateResult(stream, false, "invalid request")
		return
	}

	c.logf("[Client] Update download requested: %s", req.DownloadURL)

	// 异步执行更新
	go func() {
		if err := c.performSelfUpdate(req.DownloadURL); err != nil {
			c.logErrorf("[Client] Update failed: %v", err)
		}
	}()

	c.sendUpdateResult(stream, true, "update started")
}

// sendUpdateResult 发送更新结果
func (c *Client) sendUpdateResult(stream net.Conn, success bool, message string) {
	result := protocol.UpdateResultResponse{
		Success: success,
		Message: message,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeUpdateResult, result)
	protocol.WriteMessage(stream, msg)
}

// performSelfUpdate 执行自更新
func (c *Client) performSelfUpdate(downloadURL string) error {
	c.logf("[Client] Starting self-update from: %s", downloadURL)

	// 使用共享的下载和解压逻辑
	binaryPath, cleanup, err := update.DownloadAndExtract(downloadURL, "client")
	if err != nil {
		return err
	}
	defer cleanup()

	// 获取当前可执行文件路径
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	currentPath, _ = filepath.EvalSymlinks(currentPath)

	// Windows 需要特殊处理
	if runtime.GOOS == "windows" {
		return performWindowsClientUpdate(binaryPath, currentPath, c.ServerAddr, c.Token, c.ID)
	}

	// Linux/Mac: 直接替换
	backupPath := currentPath + ".bak"

	// 停止所有插件
	c.stopAllPlugins()

	// 备份当前文件
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fmt.Errorf("backup current: %w", err)
	}

	// 复制新文件（不能用 rename，可能跨文件系统）
	if err := update.CopyFile(binaryPath, currentPath); err != nil {
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// 设置执行权限
	if err := os.Chmod(currentPath, 0755); err != nil {
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("chmod: %w", err)
	}

	// 删除备份
	os.Remove(backupPath)

	c.logf("[Client] Update completed, restarting...")

	// 重启进程
	restartClientProcess(currentPath, c.ServerAddr, c.Token, c.ID)
	return nil
}

// stopAllPlugins 停止所有运行中的插件
func (c *Client) stopAllPlugins() {
	c.pluginMu.Lock()
	for key, handler := range c.runningPlugins {
		c.logf("[Client] Stopping plugin %s for update", key)
		handler.Stop()
	}
	c.runningPlugins = make(map[string]plugin.ClientPlugin)
	c.pluginMu.Unlock()
}

// performWindowsClientUpdate Windows 平台更新
func performWindowsClientUpdate(newFile, currentPath, serverAddr, token, id string) error {
	// 创建批处理脚本
	args := fmt.Sprintf(`-s "%s" -t "%s"`, serverAddr, token)
	if id != "" {
		args += fmt.Sprintf(` -id "%s"`, id)
	}

	batchScript := fmt.Sprintf(`@echo off
ping 127.0.0.1 -n 2 > nul
del "%s"
move "%s" "%s"
start "" "%s" %s
del "%%~f0"
`, currentPath, newFile, currentPath, currentPath, args)

	batchPath := filepath.Join(os.TempDir(), "gotunnel_client_update.bat")
	if err := os.WriteFile(batchPath, []byte(batchScript), 0755); err != nil {
		return fmt.Errorf("write batch: %w", err)
	}

	cmd := exec.Command("cmd", "/C", "start", "/MIN", batchPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start batch: %w", err)
	}

	// 退出当前进程
	os.Exit(0)
	return nil
}

// restartClientProcess 重启客户端进程
func restartClientProcess(path, serverAddr, token, id string) {
	args := []string{"-s", serverAddr, "-t", token}
	if id != "" {
		args = append(args, "-id", id)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	os.Exit(0)
}

// handlePluginStatusQuery 处理插件状态查询
func (c *Client) handlePluginStatusQuery(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	c.pluginMu.RLock()
	plugins := make([]protocol.PluginStatusEntry, 0, len(c.runningPlugins))
	for key, handler := range c.runningPlugins {
		// 从插件的 Metadata 获取真正的插件名称
		pluginName := handler.Metadata().Name
		// 如果 Metadata 没有名称，回退到从 key 解析
		if pluginName == "" {
			parts := strings.SplitN(key, ":", 2)
			pluginName = parts[0]
		}
		plugins = append(plugins, protocol.PluginStatusEntry{
			PluginName: pluginName,
			Running:    true,
		})
	}
	c.pluginMu.RUnlock()

	resp := protocol.PluginStatusQueryResponse{
		Plugins: plugins,
	}
	respMsg, _ := protocol.NewMessage(protocol.MsgTypePluginStatusQueryResp, resp)
	protocol.WriteMessage(stream, respMsg)
}

// handleLogRequest 处理日志请求
func (c *Client) handleLogRequest(stream net.Conn, msg *protocol.Message) {
	if c.logger == nil {
		stream.Close()
		return
	}

	var req protocol.LogRequest
	if err := msg.ParsePayload(&req); err != nil {
		stream.Close()
		return
	}

	c.logger.Printf("Log request received: session=%s, follow=%v", req.SessionID, req.Follow)

	// 发送历史日志
	entries := c.logger.GetRecentLogs(req.Lines, req.Level)
	if len(entries) > 0 {
		data := protocol.LogData{
			SessionID: req.SessionID,
			Entries:   entries,
			EOF:       !req.Follow,
		}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeLogData, data)
		if err := protocol.WriteMessage(stream, respMsg); err != nil {
			stream.Close()
			return
		}
	}

	// 如果不需要持续推送，关闭流
	if !req.Follow {
		stream.Close()
		return
	}

	// 订阅新日志
	ch := c.logger.Subscribe(req.SessionID)
	defer c.logger.Unsubscribe(req.SessionID)
	defer stream.Close()

	// 持续推送新日志
	for entry := range ch {
		// 应用级别过滤
		if req.Level != "" && entry.Level != req.Level {
			continue
		}

		data := protocol.LogData{
			SessionID: req.SessionID,
			Entries:   []protocol.LogEntry{entry},
			EOF:       false,
		}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeLogData, data)
		if err := protocol.WriteMessage(stream, respMsg); err != nil {
			return
		}
	}
}

// handleLogStop 处理停止日志流请求
func (c *Client) handleLogStop(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	if c.logger == nil {
		return
	}

	var req protocol.LogStopRequest
	if err := msg.ParsePayload(&req); err != nil {
		return
	}

	c.logger.Unsubscribe(req.SessionID)
}

// handlePluginAPIRequest 处理插件 API 请求
func (c *Client) handlePluginAPIRequest(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.PluginAPIRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.sendPluginAPIResponse(stream, 400, nil, "", "invalid request: "+err.Error())
		return
	}

	c.logf("[Client] Plugin API request: %s %s for plugin %s (ID: %s)", req.Method, req.Path, req.PluginName, req.PluginID)

	// 查找运行中的插件
	c.pluginMu.RLock()
	var handler plugin.ClientPlugin

	// 优先使用 PluginID 查找
	if req.PluginID != "" {
		handler = c.runningPlugins[req.PluginID]
	}

	// 如果没找到，尝试通过 PluginName 匹配（向后兼容）
	if handler == nil && req.PluginName != "" {
		for key, p := range c.runningPlugins {
			// key 可能是 PluginID 或 "pluginName:ruleName" 格式
			if strings.HasPrefix(key, req.PluginName+":") {
				handler = p
				break
			}
		}
	}
	c.pluginMu.RUnlock()

	if handler == nil {
		c.sendPluginAPIResponse(stream, 404, nil, "", "plugin not running: "+req.PluginName)
		return
	}

	// 类型断言为 JSPlugin
	jsPlugin, ok := handler.(*script.JSPlugin)
	if !ok {
		c.sendPluginAPIResponse(stream, 500, nil, "", "plugin does not support API routing")
		return
	}

	// 调用插件的 API 处理函数
	status, headers, body, err := jsPlugin.HandleAPIRequest(req.Method, req.Path, req.Query, req.Headers, req.Body)
	if err != nil {
		c.sendPluginAPIResponse(stream, 500, nil, "", err.Error())
		return
	}

	c.sendPluginAPIResponse(stream, status, headers, body, "")
}

// sendPluginAPIResponse 发送插件 API 响应
func (c *Client) sendPluginAPIResponse(stream net.Conn, status int, headers map[string]string, body, errMsg string) {
	resp := protocol.PluginAPIResponse{
		Status:  status,
		Headers: headers,
		Body:    body,
		Error:   errMsg,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypePluginAPIResponse, resp)
	protocol.WriteMessage(stream, msg)
}
