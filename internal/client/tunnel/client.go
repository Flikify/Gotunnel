package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
	"github.com/gotunnel/pkg/update"
	"github.com/gotunnel/pkg/utils"
	"github.com/gotunnel/pkg/version"
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
)

// Client 隧道客户端
type Client struct {
	ServerAddr string
	Token      string
	ID         string
	Name       string // 客户端名称（主机名）
	TLSEnabled bool
	TLSConfig  *tls.Config
	DataDir    string // 数据目录
	session    *yamux.Session
	rules      []protocol.ProxyRule
	mu         sync.RWMutex
	logger     *Logger // 日志收集器
}

// NewClient 创建客户端
func NewClient(serverAddr, token string) *Client {
	// 默认数据目录：优先使用用户主目录，失败时回退到当前工作目录
	var dataDir string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dataDir = filepath.Join(home, ".gotunnel")
	} else {
		// UserHomeDir 失败（如 Android adb shell 环境），使用当前工作目录
		if cwd, err := os.Getwd(); err == nil {
			dataDir = filepath.Join(cwd, ".gotunnel")
			log.Printf("[Client] UserHomeDir unavailable, using current directory: %s", dataDir)
		} else {
			// 最后回退到相对路径
			dataDir = ".gotunnel"
			log.Printf("[Client] Warning: using relative path for data directory")
		}
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Failed to create data dir: %v", err)
	}

	// ID 优先级：命令行参数 > 机器ID
	id := getMachineID()

	// 获取主机名作为客户端名称
	hostname, _ := os.Hostname()

	// 初始化日志收集器
	logger, err := NewLogger(dataDir)
	if err != nil {
		log.Printf("Failed to initialize logger: %v", err)
	}

	return &Client{
		ServerAddr: serverAddr,
		Token:      token,
		ID:         id,
		Name:       hostname,
		DataDir:    dataDir,
		logger:     logger,
	}
}

// InitVersionStore 初始化版本存储
func (c *Client) InitVersionStore() error {
	return nil
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
			c.logErrorf("Connect error: %v", err)
			c.logf("Reconnecting in %v...", reconnectDelay)
			time.Sleep(reconnectDelay)
			continue
		}

		c.handleSession()
		c.logWarnf("Disconnected, reconnecting...")
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

	authReq := protocol.AuthRequest{
		ClientID: c.ID,
		Token:    c.Token,
		Name:     c.Name,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  version.Version,
	}
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

	// 如果服务端分配了新 ID，则更新
	if authResp.ClientID != "" && authResp.ClientID != c.ID {
		conn.Close()
		return fmt.Errorf("server returned unexpected client id: %s", authResp.ClientID)
	}

	c.logf("Authenticated as %s", c.ID)

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
		c.handleProxyConfig(stream, msg)
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
	case protocol.MsgTypeClientRestart:
		c.handleClientRestart(stream, msg)
	case protocol.MsgTypeUpdateDownload:
		c.handleUpdateDownload(stream, msg)
	case protocol.MsgTypeLogRequest:
		go c.handleLogRequest(stream, msg)
	case protocol.MsgTypeLogStop:
		c.handleLogStop(stream, msg)
	case protocol.MsgTypeSystemStatsRequest:
		c.handleSystemStatsRequest(stream, msg)
	case protocol.MsgTypeScreenshotRequest:
		c.handleScreenshotRequest(stream, msg)
	case protocol.MsgTypeShellExecuteRequest:
		c.handleShellExecuteRequest(stream, msg)
	}
}

// handleProxyConfig 处理代理配置
func (c *Client) handleProxyConfig(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var cfg protocol.ProxyConfig
	if err := msg.ParsePayload(&cfg); err != nil {
		c.logErrorf("Parse proxy config error: %v", err)
		return
	}

	c.mu.Lock()
	c.rules = cfg.Rules
	c.mu.Unlock()

	c.logf("Received %d proxy rules", len(cfg.Rules))
	for _, r := range cfg.Rules {
		c.logf("  %s: %s:%d", r.Name, r.LocalIP, r.LocalPort)
	}

	// 发送配置确认
	ack := &protocol.Message{Type: protocol.MsgTypeProxyReady}
	protocol.WriteMessage(stream, ack)
}

// handleNewProxy 处理新代理请求
func (c *Client) handleNewProxy(stream net.Conn, msg *protocol.Message) {
	var req protocol.NewProxyRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.logErrorf("Parse new proxy request error: %v", err)
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
		c.logWarnf("Unknown port %d", req.RemotePort)
		return
	}

	localAddr := fmt.Sprintf("%s:%d", rule.LocalIP, rule.LocalPort)
	localConn, err := net.DialTimeout("tcp", localAddr, localDialTimeout)
	if err != nil {
		c.logErrorf("Connect %s error: %v", localAddr, err)
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

// handleClientRestart 处理客户端重启请求
func (c *Client) handleClientRestart(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ClientRestartRequest
	msg.ParsePayload(&req)

	c.logf("Restart requested: %s", req.Reason)

	// 发送响应
	resp := protocol.ClientRestartResponse{
		Success: true,
		Message: "restarting",
	}
	respMsg, _ := protocol.NewMessage(protocol.MsgTypeClientRestart, resp)
	protocol.WriteMessage(stream, respMsg)

	// 停止所有运行中的插件
	// 关闭会话（会触发重连）
	if c.session != nil {
		c.session.Close()
	}
}

// handleUpdateDownload 处理更新下载请求
func (c *Client) handleUpdateDownload(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.UpdateDownloadRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.logErrorf("Parse update request error: %v", err)
		c.sendUpdateResult(stream, false, "invalid request")
		return
	}

	c.logf("Update download requested: %s", req.DownloadURL)

	// 异步执行更新
	go func() {
		if err := c.performSelfUpdate(req.DownloadURL); err != nil {
			c.logErrorf("Update failed: %v", err)
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
	c.logf("Starting self-update from: %s", downloadURL)

	// 获取当前可执行文件路径
	currentPath, err := os.Executable()
	if err != nil {
		c.logErrorf("Update failed: cannot get executable path: %v", err)
		return err
	}
	currentPath, _ = filepath.EvalSymlinks(currentPath)

	// 预检查：验证是否有写权限（在下载前检查，避免浪费带宽）
	// Windows 跳过预检查，因为 Windows 更新通过 batch 脚本以提升权限执行
	// 非 Windows：原始路径 → DataDir → 临时目录，逐级回退
	fallbackDir := ""
	if runtime.GOOS != "windows" {
		if err := c.checkUpdatePermissions(currentPath); err != nil {
			// 尝试 DataDir
			fallbackDir = c.DataDir
			testFile := filepath.Join(fallbackDir, ".gotunnel_update_test")
			if f, err := os.Create(testFile); err != nil {
				// DataDir 也不可写，回退到临时目录
				fallbackDir = os.TempDir()
				c.logf("DataDir not writable, falling back to temp directory: %s", fallbackDir)
			} else {
				f.Close()
				os.Remove(testFile)
				c.logf("Original path not writable, falling back to data directory: %s", fallbackDir)
			}
		}
	}

	// 使用共享的下载和解压逻辑
	c.logf("Downloading update package...")
	binaryPath, cleanup, err := update.DownloadAndExtract(downloadURL, "client")
	if err != nil {
		c.logErrorf("Update failed: download/extract error: %v", err)
		return err
	}
	defer cleanup()

	// Windows 需要特殊处理
	if runtime.GOOS == "windows" {
		return performWindowsClientUpdate(binaryPath, currentPath, c.ServerAddr, c.Token)
	}

	// 确定目标路径
	targetPath := currentPath
	if fallbackDir != "" {
		targetPath = filepath.Join(fallbackDir, filepath.Base(currentPath))
		c.logf("Will install to fallback path: %s", targetPath)
	}

	if fallbackDir == "" {
		// 原地替换：备份 → 复制 → 清理
		backupPath := currentPath + ".bak"

		c.logf("Backing up current binary...")
		if err := os.Rename(currentPath, backupPath); err != nil {
			c.logErrorf("Update failed: cannot backup current binary: %v", err)
			return err
		}

		c.logf("Installing new binary...")
		if err := update.CopyFile(binaryPath, currentPath); err != nil {
			os.Rename(backupPath, currentPath)
			c.logErrorf("Update failed: cannot install new binary: %v", err)
			return err
		}

		if err := os.Chmod(currentPath, 0755); err != nil {
			os.Rename(backupPath, currentPath)
			c.logErrorf("Update failed: cannot set execute permission: %v", err)
			return err
		}

		os.Remove(backupPath)
	} else {
		// 回退路径：直接复制到回退目录
		c.logf("Installing new binary to data directory...")
		if err := update.CopyFile(binaryPath, targetPath); err != nil {
			c.logErrorf("Update failed: cannot install new binary: %v", err)
			return err
		}

		if err := os.Chmod(targetPath, 0755); err != nil {
			c.logErrorf("Update failed: cannot set execute permission: %v", err)
			return err
		}
	}

	c.logf("Update completed successfully, restarting...")

	// 重启进程（从新路径启动）
	restartClientProcess(targetPath, c.ServerAddr, c.Token)
	return nil
}

// checkUpdatePermissions 检查是否有更新权限
func (c *Client) checkUpdatePermissions(execPath string) error {
	// 检查可执行文件所在目录是否可写
	dir := filepath.Dir(execPath)
	testFile := filepath.Join(dir, ".gotunnel_update_test")

	f, err := os.Create(testFile)
	if err != nil {
		c.logErrorf("No write permission to directory: %s", dir)
		return err
	}
	f.Close()
	os.Remove(testFile)

	// 检查可执行文件本身是否可写
	f, err = os.OpenFile(execPath, os.O_WRONLY, 0)
	if err != nil {
		c.logErrorf("No write permission to executable: %s", execPath)
		return err
	}
	f.Close()

	return nil
}

// performWindowsClientUpdate Windows 平台更新
func performWindowsClientUpdate(newFile, currentPath, serverAddr, token string) error {
	// 创建批处理脚本
	args := fmt.Sprintf(`-s "%s" -t "%s"`, serverAddr, token)
	batchScript := fmt.Sprintf(`@echo off
:: Check for admin rights, request UAC elevation if needed
net session >nul 2>&1
if %%errorlevel%% neq 0 (
    powershell -Command "Start-Process cmd -ArgumentList '/C \\"\"%%~f0\"\"' -Verb RunAs"
    exit /b
)
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
func restartClientProcess(path, serverAddr, token string) {
	args := []string{"-s", serverAddr, "-t", token}

	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	os.Exit(0)
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

// handleSystemStatsRequest 处理系统状态请求
func (c *Client) handleSystemStatsRequest(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	stats, err := utils.GetSystemStats()
	if err != nil {
		log.Printf("Failed to get system stats: %v", err)
		return
	}

	resp := protocol.SystemStatsResponse{
		CPUUsage:    stats.CPUUsage,
		MemoryTotal: stats.MemoryTotal,
		MemoryUsed:  stats.MemoryUsed,
		MemoryUsage: stats.MemoryUsage,
		DiskTotal:   stats.DiskTotal,
		DiskUsed:    stats.DiskUsed,
		DiskUsage:   stats.DiskUsage,
	}

	respMsg, _ := protocol.NewMessage(protocol.MsgTypeSystemStatsResponse, resp)
	protocol.WriteMessage(stream, respMsg)
}

// handleScreenshotRequest 处理截图请求
func (c *Client) handleScreenshotRequest(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ScreenshotRequest
	msg.ParsePayload(&req)

	// 捕获截图
	data, width, height, err := utils.CaptureScreenshot(req.Quality)
	if err != nil {
		c.logErrorf("Screenshot capture failed: %v", err)
		resp := protocol.ScreenshotResponse{Error: err.Error()}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeScreenshotResponse, resp)
		protocol.WriteMessage(stream, respMsg)
		return
	}

	// 编码为 Base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	resp := protocol.ScreenshotResponse{
		Data:      base64Data,
		Width:     width,
		Height:    height,
		Timestamp: time.Now().UnixMilli(),
	}

	respMsg, _ := protocol.NewMessage(protocol.MsgTypeScreenshotResponse, resp)
	protocol.WriteMessage(stream, respMsg)
}

// handleShellExecuteRequest 处理 Shell 执行请求
func (c *Client) handleShellExecuteRequest(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ShellExecuteRequest
	if err := msg.ParsePayload(&req); err != nil {
		resp := protocol.ShellExecuteResponse{Error: err.Error(), ExitCode: -1}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeShellExecuteResponse, resp)
		protocol.WriteMessage(stream, respMsg)
		return
	}

	// 设置默认超时
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	c.logf("Executing shell command: %s", req.Command)

	// 根据操作系统选择 shell
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", req.Command)
	} else {
		cmd = exec.Command("sh", "-c", req.Command)
	}

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	// 执行命令并获取输出
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			resp := protocol.ShellExecuteResponse{
				Output:   string(output),
				ExitCode: -1,
				Error:    "command timeout",
			}
			respMsg, _ := protocol.NewMessage(protocol.MsgTypeShellExecuteResponse, resp)
			protocol.WriteMessage(stream, respMsg)
			return
		} else {
			resp := protocol.ShellExecuteResponse{
				Output:   string(output),
				ExitCode: -1,
				Error:    err.Error(),
			}
			respMsg, _ := protocol.NewMessage(protocol.MsgTypeShellExecuteResponse, resp)
			protocol.WriteMessage(stream, respMsg)
			return
		}
	}

	resp := protocol.ShellExecuteResponse{
		Output:   string(output),
		ExitCode: exitCode,
	}

	respMsg, _ := protocol.NewMessage(protocol.MsgTypeShellExecuteResponse, resp)
	protocol.WriteMessage(stream, respMsg)
}
