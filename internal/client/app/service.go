package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
	"github.com/gotunnel/pkg/protocol"
)

// Config captures the persisted client configuration consumed by every runtime entrypoint.
type Config struct {
	Server            string
	Token             string
	DataDir           string
	ClientID          string
	ClientName        string
	TLSEnabled        bool
	TLSConfig         *tls.Config
	Features          *tunnel.PlatformFeatures
	ReconnectDelay    time.Duration
	ReconnectMaxDelay time.Duration
}

// ActiveTunnel mirrors the active proxy listeners currently established for the client.
type ActiveTunnel struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	RemotePort  int    `json:"remote_port"`
	LocalIP     string `json:"local_ip,omitempty"`
	LocalPort   int    `json:"local_port"`
	Status      string `json:"status,omitempty"`
	ConnectedAt int64  `json:"connected_at"`
}

// Snapshot is the coarse-grained runtime state exposed to CLI, mobile, and future adapters.
type Snapshot struct {
	IsRunning     bool           `json:"is_running"`
	Status        string         `json:"status"`
	Detail        string         `json:"detail"`
	LastError     string         `json:"last_error"`
	RecentLogs    string         `json:"recent_logs"`
	ActiveTunnels []ActiveTunnel `json:"active_tunnels,omitempty"`
}

// Service owns client runtime lifecycle and in-process state publication.
type Service struct {
	mu sync.Mutex

	config Config

	client              *tunnel.Client
	cancel              context.CancelFunc
	cancelLogs          func()
	running             bool
	status              string
	detail              string
	lastError           string
	recentLogs          []string
	tunnelEstablishedAt map[string]int64
}

// NewService creates a reusable client application service.
func NewService() *Service {
	return &Service{
		status:              "stopped",
		detail:              "stopped",
		tunnelEstablishedAt: make(map[string]int64),
	}
}

// Configure stores the latest desired runtime configuration.
func (s *Service) Configure(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// RunContext starts the configured client runtime and blocks until it exits or ctx is cancelled.
func (s *Service) RunContext(ctx context.Context) error {
	client, err := s.prepareLocked(nil)
	if err != nil {
		return err
	}

	err = client.RunContext(ctx)
	s.finish(err)
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// Start runs the configured client runtime in the background.
func (s *Service) Start() string {
	ctx, cancel := context.WithCancel(context.Background())
	client, err := s.prepareLocked(cancel)
	if err != nil {
		cancel()
		return err.Error()
	}

	go func() {
		err := client.RunContext(ctx)
		s.finish(err)
	}()
	return ""
}

// Stop cancels the active runtime if one exists.
func (s *Service) Stop() string {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.running = false
	s.status = "stopped"
	s.detail = "stopped"
	s.tunnelEstablishedAt = make(map[string]int64)
	if s.cancelLogs != nil {
		s.cancelLogs()
		s.cancelLogs = nil
	}
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return ""
}

// Restart stops the current runtime and starts a new one with the stored config.
func (s *Service) Restart() string {
	s.Stop()
	return s.Start()
}

// Snapshot returns the latest coarse-grained state for UI adapters.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return Snapshot{
		IsRunning:     s.running,
		Status:        s.status,
		Detail:        s.detail,
		LastError:     s.lastError,
		RecentLogs:    strings.Join(s.recentLogs, "\n"),
		ActiveTunnels: s.activeTunnelsLocked(),
	}
}

func (s *Service) prepareLocked(cancel context.CancelFunc) (*tunnel.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil, fmt.Errorf("client runtime already running")
	}
	if strings.TrimSpace(s.config.Server) == "" || strings.TrimSpace(s.config.Token) == "" {
		return nil, fmt.Errorf("server and token are required")
	}

	client := tunnel.NewClientWithOptions(s.config.Server, s.config.Token, tunnel.ClientOptions{
		DataDir:           s.config.DataDir,
		ClientID:          s.config.ClientID,
		ClientName:        s.config.ClientName,
		Features:          s.config.Features,
		ReconnectDelay:    s.config.ReconnectDelay,
		ReconnectMaxDelay: s.config.ReconnectMaxDelay,
	})
	if s.config.TLSEnabled {
		client.TLSEnabled = true
		client.TLSConfig = s.config.TLSConfig
		if client.TLSConfig == nil {
			client.TLSConfig = crypto.ClientTLSConfig()
		}
	}

	if s.cancelLogs != nil {
		s.cancelLogs()
	}
	s.client = client
	s.cancel = cancel
	s.cancelLogs = client.ObserveLogs(s.consumeLogEntry)
	s.running = true
	s.status = "starting"
	s.detail = fmt.Sprintf("Starting client for %s", s.config.Server)
	s.lastError = ""
	s.tunnelEstablishedAt = make(map[string]int64)
	return client, nil
}

func (s *Service) finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	s.cancel = nil
	s.client = nil
	if s.cancelLogs != nil {
		s.cancelLogs()
		s.cancelLogs = nil
	}

	if err != nil {
		s.status = "error"
		s.detail = err.Error()
		s.lastError = err.Error()
		s.appendLogLocked("ERROR", err.Error(), time.Now().UnixMilli())
		return
	}

	if s.status != "stopped" {
		s.status = "stopped"
		s.detail = "stopped"
	}
}

func (s *Service) consumeLogEntry(entry protocol.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appendLogLocked(entry.Level, entry.Message, entry.Timestamp)

	lower := strings.ToLower(entry.Message)
	switch {
	case strings.HasPrefix(lower, "dialing server"),
		strings.HasPrefix(lower, "tcp connection established"),
		strings.HasPrefix(lower, "starting tls handshake"),
		strings.HasPrefix(lower, "tls handshake completed"),
		strings.HasPrefix(lower, "sending auth request"),
		strings.HasPrefix(lower, "server authentication accepted"),
		strings.HasPrefix(lower, "tunnel session established"):
		if s.status != "running" {
			s.status = "starting"
		}
		s.detail = entry.Message
	case strings.HasPrefix(lower, "authenticated as"):
		s.status = "running"
		s.detail = entry.Message
		s.lastError = ""
	case strings.HasPrefix(lower, "connect error:"):
		s.status = "reconnecting"
		s.detail = entry.Message
		s.lastError = entry.Message
	case strings.Contains(lower, "auth failed:"):
		s.status = "error"
		s.detail = entry.Message
		s.lastError = entry.Message
	case strings.Contains(lower, "reconnecting"):
		s.status = "reconnecting"
		s.detail = entry.Message
	case strings.Contains(lower, "disconnected"):
		s.status = "reconnecting"
		s.detail = entry.Message
	case entry.Level == "error":
		s.status = "error"
		s.detail = entry.Message
		s.lastError = entry.Message
	case s.status == "starting":
		s.detail = entry.Message
	}
}

func (s *Service) appendLogLocked(level, message string, ts int64) {
	if strings.TrimSpace(message) == "" {
		return
	}

	stamp := time.UnixMilli(ts)
	if ts <= 0 {
		stamp = time.Now()
	}

	line := fmt.Sprintf("%s [%s] %s", stamp.Format("15:04:05"), strings.ToUpper(level), message)
	s.recentLogs = append(s.recentLogs, line)
	if len(s.recentLogs) > 80 {
		s.recentLogs = s.recentLogs[len(s.recentLogs)-80:]
	}
}

func (s *Service) activeTunnelsLocked() []ActiveTunnel {
	if s.client == nil || s.status != "running" {
		s.tunnelEstablishedAt = make(map[string]int64)
		return nil
	}

	rules := s.client.RulesSnapshot()
	if len(rules) == 0 {
		s.tunnelEstablishedAt = make(map[string]int64)
		return nil
	}

	now := time.Now().UnixMilli()
	activeKeys := make(map[string]struct{}, len(rules))
	tunnels := make([]ActiveTunnel, 0, len(rules))
	for _, rule := range rules {
		if !rule.IsEnabled() || rule.PortStatus != "listening" {
			continue
		}

		key := activeTunnelKey(rule)
		activeKeys[key] = struct{}{}

		connectedAt := s.tunnelEstablishedAt[key]
		if connectedAt == 0 {
			connectedAt = now
			s.tunnelEstablishedAt[key] = connectedAt
		}

		tunnels = append(tunnels, ActiveTunnel{
			Name:        rule.Name,
			Type:        rule.Type,
			RemotePort:  rule.RemotePort,
			LocalIP:     rule.LocalIP,
			LocalPort:   rule.LocalPort,
			Status:      rule.PortStatus,
			ConnectedAt: connectedAt,
		})
	}

	for key := range s.tunnelEstablishedAt {
		if _, ok := activeKeys[key]; !ok {
			delete(s.tunnelEstablishedAt, key)
		}
	}

	sort.Slice(tunnels, func(i, j int) bool {
		if tunnels[i].RemotePort == tunnels[j].RemotePort {
			return tunnels[i].LocalPort < tunnels[j].LocalPort
		}
		return tunnels[i].RemotePort < tunnels[j].RemotePort
	})
	return tunnels
}

func activeTunnelKey(rule protocol.ProxyRule) string {
	return fmt.Sprintf("%s|%s|%d|%s|%d|%s",
		rule.Name,
		rule.Type,
		rule.RemotePort,
		rule.LocalIP,
		rule.LocalPort,
		rule.PortStatus,
	)
}
