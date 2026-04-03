//go:build windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	clientconfig "github.com/gotunnel/internal/client/config"
	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
	"golang.org/x/sys/windows"
)

const (
	desktopHelperDialTimeout   = 2 * time.Second
	desktopHelperLaunchTimeout = 8 * time.Second
	windowsInvalidSessionID    = 0xFFFFFFFF
)

type serviceRemoteOpsProxy struct {
	dataDir string
	mu      sync.Mutex
}

func NewServiceRemoteOpsProxy(dataDir string) tunnel.RemoteOpsProxy {
	return &serviceRemoteOpsProxy{dataDir: normalizeDataDir(dataDir)}
}

func RunHelper(ctx context.Context, dataDir string, sessionID uint32) error {
	if sessionID == 0 {
		return fmt.Errorf("invalid helper session id")
	}
	dataDir = normalizeDataDir(dataDir)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create helper data dir: %w", err)
	}

	socketPath := helperSocketPath(dataDir, sessionID)
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen helper socket: %w", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	helperClient, err := newHelperClient(dataDir)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			continue
		}
		go helperClient.HandleControlConnection(conn)
	}
}

func newHelperClient(dataDir string) (*tunnel.Client, error) {
	cfgPath := filepath.Join(dataDir, "client.yaml")
	cfg, err := clientconfig.LoadClientConfig(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load helper config: %w", err)
	}
	if cfg == nil {
		cfg = &clientconfig.ClientConfig{}
	}

	return tunnel.NewClientWithOptions("", "", tunnel.ClientOptions{
		DataDir:    dataDir,
		ClientID:   cfg.ClientID,
		ClientName: cfg.Name,
	}), nil
}

func (p *serviceRemoteOpsProxy) ProxyScreenshot(stream net.Conn, msg *protocol.Message) error {
	defer stream.Close()

	conn, err := p.connectHelper()
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := protocol.WriteMessage(conn, msg); err != nil {
		return fmt.Errorf("forward screenshot request: %w", err)
	}

	resp, err := protocol.ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("read screenshot response: %w", err)
	}
	return protocol.WriteMessage(stream, resp)
}

func (p *serviceRemoteOpsProxy) ProxyRemoteControl(stream net.Conn, msg *protocol.Message) error {
	conn, err := p.connectHelper()
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := protocol.WriteMessage(conn, msg); err != nil {
		return fmt.Errorf("forward remote control start: %w", err)
	}

	relay.Relay(stream, conn)
	return nil
}

func (p *serviceRemoteOpsProxy) connectHelper() (net.Conn, error) {
	sessionID, err := activeConsoleSessionID()
	if err != nil {
		return nil, err
	}

	socketPath := helperSocketPath(p.dataDir, sessionID)
	if conn, err := dialHelperSocket(socketPath); err == nil {
		return conn, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, err := dialHelperSocket(socketPath); err == nil {
		return conn, nil
	}

	if err := launchHelperProcess(p.dataDir, sessionID); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(desktopHelperLaunchTimeout)
	for time.Now().Before(deadline) {
		conn, err := dialHelperSocket(socketPath)
		if err == nil {
			return conn, nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return nil, fmt.Errorf("desktop helper did not become ready in session %d", sessionID)
}

func dialHelperSocket(socketPath string) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, desktopHelperDialTimeout)
}

func activeConsoleSessionID() (uint32, error) {
	sessionID := windows.WTSGetActiveConsoleSessionId()
	if sessionID == windowsInvalidSessionID {
		return 0, fmt.Errorf("no active user session is available for desktop operations")
	}
	return sessionID, nil
}

func helperSocketPath(dataDir string, sessionID uint32) string {
	baseDir := normalizeDataDir(dataDir)
	return filepath.Join(baseDir, "desktop-helper-"+strconv.FormatUint(uint64(sessionID), 10)+".sock")
}

func normalizeDataDir(dataDir string) string {
	baseDir := strings.TrimSpace(dataDir)
	if baseDir == "" {
		baseDir = `C:\ProgramData\GoTunnel`
	}
	return baseDir
}

func launchHelperProcess(dataDir string, sessionID uint32) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve helper executable: %w", err)
	}

	var userToken windows.Token
	if err := windows.WTSQueryUserToken(sessionID, &userToken); err != nil {
		return fmt.Errorf("query active session token: %w", err)
	}
	defer userToken.Close()

	var primaryToken windows.Token
	if err := windows.DuplicateTokenEx(userToken, windows.MAXIMUM_ALLOWED, nil, windows.SecurityImpersonation, windows.TokenPrimary, &primaryToken); err != nil {
		return fmt.Errorf("duplicate active session token: %w", err)
	}
	defer primaryToken.Close()

	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, primaryToken, false); err != nil {
		return fmt.Errorf("create helper environment: %w", err)
	}
	defer windows.DestroyEnvironmentBlock(env)

	desktopPtr, err := windows.UTF16PtrFromString("winsta0\\default")
	if err != nil {
		return fmt.Errorf("encode helper desktop: %w", err)
	}
	commandLine, err := windows.UTF16PtrFromString(buildHelperCommandLine(exePath, dataDir, sessionID))
	if err != nil {
		return fmt.Errorf("encode helper command line: %w", err)
	}
	appName, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return fmt.Errorf("encode helper path: %w", err)
	}
	currentDir, err := windows.UTF16PtrFromString(filepath.Dir(exePath))
	if err != nil {
		return fmt.Errorf("encode helper cwd: %w", err)
	}

	si := windows.StartupInfo{
		Cb:         uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop:    desktopPtr,
		Flags:      windows.STARTF_USESHOWWINDOW,
		ShowWindow: windows.SW_HIDE,
	}
	var pi windows.ProcessInformation
	if err := windows.CreateProcessAsUser(
		primaryToken,
		appName,
		commandLine,
		nil,
		nil,
		false,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW,
		env,
		currentDir,
		&si,
		&pi,
	); err != nil {
		return fmt.Errorf("launch desktop helper: %w", err)
	}
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)
	return nil
}

func buildHelperCommandLine(exePath, dataDir string, sessionID uint32) string {
	args := []string{
		exePath,
		"desktop-helper",
		"-data-dir", dataDir,
		"-helper-session", strconv.FormatUint(uint64(sessionID), 10),
	}
	escaped := make([]string, 0, len(args))
	for _, arg := range args {
		escaped = append(escaped, syscall.EscapeArg(arg))
	}
	return strings.Join(escaped, " ")
}
