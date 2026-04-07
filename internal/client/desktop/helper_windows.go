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
	"time"
	"unsafe"

	clientconfig "github.com/gotunnel/internal/client/config"
	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
	"golang.org/x/sys/windows"
)

const (
	desktopHelperDialTimeout = 2 * time.Second
	windowsInvalidSessionID  = 0xFFFFFFFF
)

type serviceRemoteOpsProxy struct {
	dataDir string
	mu      sync.Mutex
}

func NewServiceRemoteOpsProxy(dataDir string) tunnel.RemoteOpsProxy {
	return &serviceRemoteOpsProxy{dataDir: normalizeDataDir(dataDir)}
}

func RunHelper(ctx context.Context, dataDir string, sessionID uint32) error {
	dataDir = normalizeDataDir(dataDir)
	if sessionID == 0 {
		var err error
		sessionID, err = currentProcessSessionID()
		if err != nil {
			return fmt.Errorf("resolve desktop agent session: %w", err)
		}
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create helper data dir: %w", err)
	}

	socketPath := helperSocketPath(dataDir, sessionID)
	if conn, err := dialHelperSocket(socketPath); err == nil {
		conn.Close()
		<-ctx.Done()
		return nil
	}
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
	conn, err := p.connectHelper()
	if err != nil {
		return err
	}
	defer conn.Close()
	defer stream.Close()

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
	defer stream.Close()

	if err := protocol.WriteMessage(conn, msg); err != nil {
		return fmt.Errorf("forward remote control start: %w", err)
	}

	relay.Relay(stream, conn)
	return nil
}

func (p *serviceRemoteOpsProxy) connectHelper() (net.Conn, error) {
	candidates := interactiveSessionCandidates()
	for _, sessionID := range candidates {
		socketPath := helperSocketPath(p.dataDir, sessionID)
		if conn, err := dialHelperSocket(socketPath); err == nil {
			return conn, nil
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, sessionID := range candidates {
		socketPath := helperSocketPath(p.dataDir, sessionID)
		if conn, err := dialHelperSocket(socketPath); err == nil {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("desktop is unavailable; ensure a Windows user session is signed in and the desktop agent is running")
}

func dialHelperSocket(socketPath string) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, desktopHelperDialTimeout)
}

func interactiveSessionCandidates() []uint32 {
	candidates := make([]uint32, 0, 8)
	consoleSessionID := windows.WTSGetActiveConsoleSessionId()
	if consoleSessionID != windowsInvalidSessionID {
		candidates = append(candidates, consoleSessionID)
	}

	var sessions *windows.WTS_SESSION_INFO
	var count uint32
	if err := windows.WTSEnumerateSessions(0, 0, 1, &sessions, &count); err == nil {
		defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(sessions)))

		entries := unsafe.Slice(sessions, count)
		for _, session := range entries {
			if session.State != windows.WTSActive && session.State != windows.WTSConnected {
				continue
			}
			if !containsSessionID(candidates, session.SessionID) {
				candidates = append(candidates, session.SessionID)
			}
		}
	}
	return candidates
}

func containsSessionID(ids []uint32, target uint32) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
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

func currentProcessSessionID() (uint32, error) {
	var sessionID uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &sessionID); err != nil {
		return 0, err
	}
	return sessionID, nil
}
