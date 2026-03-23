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

const (
	dialTimeout       = 10 * time.Second
	localDialTimeout  = 5 * time.Second
	udpTimeout        = 10 * time.Second
	reconnectDelay    = 5 * time.Second
	maxReconnectDelay = 30 * time.Second
	disconnectDelay   = 3 * time.Second
	tcpKeepAlive      = 30 * time.Second
	udpBufferSize     = 65535
)

// Client is the tunnel client runtime.
type Client struct {
	ServerAddr string
	Token      string
	ID         string
	Name       string
	TLSEnabled bool
	TLSConfig  *tls.Config
	DataDir    string

	features          PlatformFeatures
	reconnectDelay    time.Duration
	reconnectMaxDelay time.Duration

	session *yamux.Session
	rules   []protocol.ProxyRule
	mu      sync.RWMutex
	logger  *Logger
}

// NewClient creates a client with default desktop options.
func NewClient(serverAddr, token string) *Client {
	return NewClientWithOptions(serverAddr, token, ClientOptions{})
}

// NewClientWithOptions creates a client with explicit runtime options.
func NewClientWithOptions(serverAddr, token string, opts ClientOptions) *Client {
	dataDir := resolveDataDir(opts.DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Failed to create data dir: %v", err)
	}

	logger, err := NewLogger(dataDir)
	if err != nil {
		log.Printf("Failed to initialize logger: %v", err)
	}

	features := DefaultPlatformFeatures()
	if opts.Features != nil {
		features = *opts.Features
	}

	delay := opts.ReconnectDelay
	if delay <= 0 {
		delay = reconnectDelay
	}

	maxDelay := opts.ReconnectMaxDelay
	if maxDelay <= 0 {
		maxDelay = maxReconnectDelay
	}
	if maxDelay < delay {
		maxDelay = delay
	}

	return &Client{
		ServerAddr:        serverAddr,
		Token:             token,
		ID:                resolveClientID(dataDir, opts.ClientID),
		Name:              resolveClientName(opts.ClientName),
		DataDir:           dataDir,
		features:          features,
		reconnectDelay:    delay,
		reconnectMaxDelay: maxDelay,
		logger:            logger,
	}
}

// InitVersionStore is kept for compatibility with older callers.
func (c *Client) InitVersionStore() error {
	return nil
}

func (c *Client) logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if c.logger != nil {
		c.logger.Printf("%s", msg)
	}
}

func (c *Client) logErrorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if c.logger != nil {
		c.logger.Errorf("%s", msg)
	}
}

func (c *Client) logWarnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if c.logger != nil {
		c.logger.Warnf("%s", msg)
	}
}

// ObserveLogs subscribes an in-process callback to future client log entries.
func (c *Client) ObserveLogs(fn func(protocol.LogEntry)) func() {
	if c.logger == nil || fn == nil {
		return func() {}
	}
	return c.logger.AddObserver(fn)
}

// RulesSnapshot returns a copy of the latest proxy rules pushed by the server.
func (c *Client) RulesSnapshot() []protocol.ProxyRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.rules) == 0 {
		return nil
	}

	rules := make([]protocol.ProxyRule, len(c.rules))
	copy(rules, c.rules)
	return rules
}

// Run starts the reconnect loop until the process exits.
func (c *Client) Run() error {
	return c.RunContext(context.Background())
}

// RunContext starts the reconnect loop and exits when ctx is cancelled.
func (c *Client) RunContext(ctx context.Context) error {
	backoff := c.reconnectDelay

	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := c.connect(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logErrorf("Connect error: %v", err)
			c.logf("Reconnecting in %v...", backoff)
			if !sleepWithContext(ctx, backoff) {
				return nil
			}
			backoff *= 2
			if backoff > c.reconnectMaxDelay {
				backoff = c.reconnectMaxDelay
			}
			continue
		}

		backoff = c.reconnectDelay
		c.handleSession(ctx)
		if ctx.Err() != nil {
			return nil
		}
		c.logWarnf("Disconnected, reconnecting...")
		if !sleepWithContext(ctx, disconnectDelay) {
			return nil
		}
	}
}

func sleepWithContext(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (c *Client) connect(ctx context.Context) error {
	var conn net.Conn
	var err error

	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: tcpKeepAlive,
	}

	c.logf("Dialing server %s (tls=%t)", c.ServerAddr, c.TLSEnabled && c.TLSConfig != nil)

	if c.TLSEnabled && c.TLSConfig != nil {
		rawConn, dialErr := dialer.DialContext(ctx, "tcp", c.ServerAddr)
		if dialErr != nil {
			return fmt.Errorf("dial server %s: %w", c.ServerAddr, dialErr)
		}
		c.logf("TCP connection established to %s", c.ServerAddr)
		c.logf("Starting TLS handshake with %s", c.ServerAddr)
		tlsConn := tls.Client(rawConn, c.TLSConfig)
		if handshakeErr := tlsConn.HandshakeContext(ctx); handshakeErr != nil {
			rawConn.Close()
			return fmt.Errorf("tls handshake with %s: %w", c.ServerAddr, handshakeErr)
		}
		state := tlsConn.ConnectionState()
		c.logf(
			"TLS handshake completed with %s using %s / %s",
			c.ServerAddr,
			tls.VersionName(state.Version),
			tls.CipherSuiteName(state.CipherSuite),
		)
		conn = tlsConn
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", c.ServerAddr)
		if err == nil {
			c.logf("TCP connection established to %s without TLS", c.ServerAddr)
		}
	}
	if err != nil {
		return fmt.Errorf("dial server %s: %w", c.ServerAddr, err)
	}

	authReq := protocol.AuthRequest{
		ClientID: c.ID,
		Token:    c.Token,
		Name:     c.Name,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  version.Version,
	}
	c.logf("Sending auth request as %s (%s/%s, version=%s)", c.ID, runtime.GOOS, runtime.GOARCH, version.Version)
	msg, _ := protocol.NewMessage(protocol.MsgTypeAuth, authReq)
	if err := protocol.WriteMessage(conn, msg); err != nil {
		conn.Close()
		return fmt.Errorf("write auth request: %w", err)
	}

	resp, err := protocol.ReadMessage(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("read auth response: %w", err)
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
	if authResp.ClientID != "" && authResp.ClientID != c.ID {
		conn.Close()
		return fmt.Errorf("server returned unexpected client id: %s", authResp.ClientID)
	}

	c.logf("Server authentication accepted for %s", c.ID)
	c.logf("Authenticated as %s", c.ID)

	session, err := yamux.Client(conn, nil)
	if err != nil {
		conn.Close()
		return fmt.Errorf("open yamux session: %w", err)
	}

	c.mu.Lock()
	c.session = session
	c.mu.Unlock()

	c.logf("Tunnel session established with %s", c.ServerAddr)

	return nil
}

func (c *Client) currentSession() *yamux.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

func (c *Client) handleSession(ctx context.Context) {
	session := c.currentSession()
	if session == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-done:
		}
	}()
	defer close(done)
	defer session.Close()

	for {
		stream, err := session.Accept()
		if err != nil {
			return
		}
		go c.handleStream(stream)
	}
}

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
	default:
		stream.Close()
	}
}

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

	ack := &protocol.Message{Type: protocol.MsgTypeProxyReady}
	protocol.WriteMessage(stream, ack)
}

func (c *Client) handleNewProxy(stream net.Conn, msg *protocol.Message) {
	var req protocol.NewProxyRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.logErrorf("Parse new proxy request error: %v", err)
		return
	}

	var rule *protocol.ProxyRule
	c.mu.RLock()
	for i := range c.rules {
		if c.rules[i].RemotePort == req.RemotePort {
			rule = &c.rules[i]
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
		c.sendProxyResult(stream, false, err.Error())
		return
	}
	defer localConn.Close()

	if err := c.sendProxyResult(stream, true, ""); err != nil {
		return
	}

	relay.Relay(stream, localConn)
}

func (c *Client) handleHeartbeat(stream net.Conn) {
	msg := &protocol.Message{Type: protocol.MsgTypeHeartbeatAck}
	protocol.WriteMessage(stream, msg)
}

func (c *Client) handleProxyConnect(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ProxyConnectRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.sendProxyResult(stream, false, "invalid request")
		return
	}

	targetConn, err := net.DialTimeout("tcp", req.Target, dialTimeout)
	if err != nil {
		c.sendProxyResult(stream, false, err.Error())
		return
	}
	defer targetConn.Close()

	if err := c.sendProxyResult(stream, true, ""); err != nil {
		return
	}

	relay.Relay(stream, targetConn)
}

func (c *Client) sendProxyResult(stream net.Conn, success bool, message string) error {
	result := protocol.ProxyConnectResult{Success: success, Message: message}
	msg, _ := protocol.NewMessage(protocol.MsgTypeProxyResult, result)
	return protocol.WriteMessage(stream, msg)
}

func (c *Client) handleUDPData(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var packet protocol.UDPPacket
	if err := msg.ParsePayload(&packet); err != nil {
		return
	}

	rule := c.findRuleByPort(packet.RemotePort)
	if rule == nil {
		return
	}

	target := fmt.Sprintf("%s:%d", rule.LocalIP, rule.LocalPort)
	conn, err := net.DialTimeout("udp", target, localDialTimeout)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(udpTimeout))
	if _, err := conn.Write(packet.Data); err != nil {
		return
	}

	buf := make([]byte, udpBufferSize)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	respPacket := protocol.UDPPacket{
		RemotePort: packet.RemotePort,
		ClientAddr: packet.ClientAddr,
		Data:       buf[:n],
	}
	respMsg, _ := protocol.NewMessage(protocol.MsgTypeUDPData, respPacket)
	protocol.WriteMessage(stream, respMsg)
}

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

func (c *Client) handleClientRestart(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ClientRestartRequest
	msg.ParsePayload(&req)

	c.logf("Restart requested: %s", req.Reason)

	resp := protocol.ClientRestartResponse{
		Success: true,
		Message: "restarting",
	}
	respMsg, _ := protocol.NewMessage(protocol.MsgTypeClientRestart, resp)
	protocol.WriteMessage(stream, respMsg)

	if session := c.currentSession(); session != nil {
		session.Close()
	}
}

func (c *Client) handleUpdateDownload(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	if !c.features.AllowSelfUpdate {
		c.sendUpdateResult(stream, false, "self-update not supported on this platform")
		return
	}

	var req protocol.UpdateDownloadRequest
	if err := msg.ParsePayload(&req); err != nil {
		c.logErrorf("Parse update request error: %v", err)
		c.sendUpdateResult(stream, false, "invalid request")
		return
	}

	c.logf("Update download requested: %s", req.DownloadURL)

	go func() {
		if err := c.performSelfUpdate(req.DownloadURL); err != nil {
			c.logErrorf("Update failed: %v", err)
		}
	}()

	c.sendUpdateResult(stream, true, "update started")
}

func (c *Client) sendUpdateResult(stream net.Conn, success bool, message string) {
	result := protocol.UpdateResultResponse{
		Success: success,
		Message: message,
	}
	msg, _ := protocol.NewMessage(protocol.MsgTypeUpdateResult, result)
	protocol.WriteMessage(stream, msg)
}

func (c *Client) performSelfUpdate(downloadURL string) error {
	if runtime.GOOS == "android" {
		return fmt.Errorf("self-update must be handled by the Android host app")
	}

	c.logf("Starting self-update from: %s", downloadURL)

	currentPath, err := os.Executable()
	if err != nil {
		c.logErrorf("Update failed: cannot get executable path: %v", err)
		return err
	}
	currentPath, _ = filepath.EvalSymlinks(currentPath)

	fallbackDir := ""
	if runtime.GOOS != "windows" {
		if err := c.checkUpdatePermissions(currentPath); err != nil {
			fallbackDir = c.DataDir
			testFile := filepath.Join(fallbackDir, ".gotunnel_update_test")
			if f, err := os.Create(testFile); err != nil {
				fallbackDir = os.TempDir()
				c.logf("DataDir not writable, falling back to temp directory: %s", fallbackDir)
			} else {
				f.Close()
				os.Remove(testFile)
				c.logf("Original path not writable, falling back to data directory: %s", fallbackDir)
			}
		}
	}

	c.logf("Downloading update package...")
	binaryPath, cleanup, err := update.DownloadAndExtract(downloadURL, "client")
	if err != nil {
		c.logErrorf("Update failed: download/extract error: %v", err)
		return err
	}
	defer cleanup()

	if runtime.GOOS == "windows" {
		return performWindowsClientUpdate(binaryPath, currentPath, c.ServerAddr, c.Token)
	}

	targetPath := currentPath
	if fallbackDir != "" {
		targetPath = filepath.Join(fallbackDir, filepath.Base(currentPath))
		c.logf("Will install to fallback path: %s", targetPath)
	}

	if fallbackDir == "" {
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
	restartClientProcess(targetPath, c.ServerAddr, c.Token)
	return nil
}

func (c *Client) checkUpdatePermissions(execPath string) error {
	dir := filepath.Dir(execPath)
	testFile := filepath.Join(dir, ".gotunnel_update_test")

	f, err := os.Create(testFile)
	if err != nil {
		c.logErrorf("No write permission to directory: %s", dir)
		return err
	}
	f.Close()
	os.Remove(testFile)

	f, err = os.OpenFile(execPath, os.O_WRONLY, 0)
	if err != nil {
		c.logErrorf("No write permission to executable: %s", execPath)
		return err
	}
	f.Close()

	return nil
}

func performWindowsClientUpdate(newFile, currentPath, serverAddr, token string) error {
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

	os.Exit(0)
	return nil
}

func restartClientProcess(path, serverAddr, token string) {
	args := []string{"-s", serverAddr, "-t", token}

	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	os.Exit(0)
}

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

	if !req.Follow {
		stream.Close()
		return
	}

	ch := c.logger.Subscribe(req.SessionID)
	defer c.logger.Unsubscribe(req.SessionID)
	defer stream.Close()

	for entry := range ch {
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

func (c *Client) handleSystemStatsRequest(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	if !c.features.AllowSystemStats {
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeSystemStatsResponse, protocol.SystemStatsResponse{})
		protocol.WriteMessage(stream, respMsg)
		return
	}

	stats, err := utils.GetSystemStats()
	if err != nil {
		log.Printf("Failed to get system stats: %v", err)
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeSystemStatsResponse, protocol.SystemStatsResponse{})
		protocol.WriteMessage(stream, respMsg)
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

func (c *Client) handleScreenshotRequest(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	var req protocol.ScreenshotRequest
	msg.ParsePayload(&req)

	if !c.features.AllowScreenshot {
		resp := protocol.ScreenshotResponse{Error: "screenshot not supported on this platform"}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeScreenshotResponse, resp)
		protocol.WriteMessage(stream, respMsg)
		return
	}

	data, width, height, err := utils.CaptureScreenshot(req.Quality)
	if err != nil {
		c.logErrorf("Screenshot capture failed: %v", err)
		resp := protocol.ScreenshotResponse{Error: err.Error()}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeScreenshotResponse, resp)
		protocol.WriteMessage(stream, respMsg)
		return
	}

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

func (c *Client) handleShellExecuteRequest(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	if !c.features.AllowShellExecute {
		resp := protocol.ShellExecuteResponse{ExitCode: -1, Error: "remote shell execution not supported on this platform"}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeShellExecuteResponse, resp)
		protocol.WriteMessage(stream, respMsg)
		return
	}

	var req protocol.ShellExecuteRequest
	if err := msg.ParsePayload(&req); err != nil {
		resp := protocol.ShellExecuteResponse{Error: err.Error(), ExitCode: -1}
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeShellExecuteResponse, resp)
		protocol.WriteMessage(stream, respMsg)
		return
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	c.logf("Executing shell command: %s", req.Command)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", req.Command)
	} else {
		cmd = exec.Command("sh", "-c", req.Command)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

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
