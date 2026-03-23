package gotunnelmobile

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
	"github.com/gotunnel/pkg/protocol"
)

// Service exposes a gomobile-friendly wrapper around the Go tunnel client.
type Service struct {
	mu sync.Mutex

	server     string
	token      string
	dataDir    string
	clientName string
	clientID   string
	disableTLS bool

	client     *tunnel.Client
	cancel     context.CancelFunc
	running    bool
	status     string
	detail     string
	lastError  string
	recentLogs []string
	cancelLogs func()

	tunnelEstablishedAt map[string]int64
}

// ActiveTunnel describes a proxy rule that is currently active on the server.
type ActiveTunnel struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	RemotePort  int    `json:"remote_port"`
	LocalIP     string `json:"local_ip,omitempty"`
	LocalPort   int    `json:"local_port"`
	Status      string `json:"status,omitempty"`
	ConnectedAt int64  `json:"connected_at"`
}

// NewService creates a mobile client service wrapper.
func NewService() *Service {
	return &Service{
		status:              "stopped",
		detail:              "stopped",
		tunnelEstablishedAt: make(map[string]int64),
	}
}

// Configure stores the parameters used by Start.
func (s *Service) Configure(server, token, dataDir, clientName, clientID string, disableTLS bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.server = strings.TrimSpace(server)
	s.token = strings.TrimSpace(token)
	s.dataDir = strings.TrimSpace(dataDir)
	s.clientName = strings.TrimSpace(clientName)
	s.clientID = strings.TrimSpace(clientID)
	s.disableTLS = disableTLS
}

// Start launches the tunnel loop in the background.
func (s *Service) Start() string {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ""
	}
	if s.server == "" || s.token == "" {
		s.mu.Unlock()
		return "server and token are required"
	}

	features := tunnel.MobilePlatformFeatures()
	client := tunnel.NewClientWithOptions(s.server, s.token, tunnel.ClientOptions{
		DataDir:    s.dataDir,
		ClientID:   s.clientID,
		ClientName: s.clientName,
		Features:   &features,
	})
	if !s.disableTLS {
		client.TLSEnabled = true
		client.TLSConfig = crypto.ClientTLSConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.client = client
	s.cancel = cancel
	s.running = true
	s.status = "starting"
	s.detail = fmt.Sprintf("Starting native client for %s", s.server)
	s.lastError = ""
	s.tunnelEstablishedAt = make(map[string]int64)
	if s.cancelLogs != nil {
		s.cancelLogs()
	}
	s.cancelLogs = client.ObserveLogs(s.consumeLogEntry)
	s.mu.Unlock()

	go func() {
		err := client.RunContext(ctx)

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
	}()

	return ""
}

// Stop cancels the running tunnel loop.
func (s *Service) Stop() string {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.running = false
	s.status = "stopped"
	s.detail = "stopped"
	s.tunnelEstablishedAt = make(map[string]int64)
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	s.mu.Lock()
	if s.cancelLogs != nil {
		s.cancelLogs()
		s.cancelLogs = nil
	}
	s.mu.Unlock()

	return ""
}

// Restart restarts the service with the stored configuration.
func (s *Service) Restart() string {
	s.Stop()
	return s.Start()
}

// IsRunning reports whether the tunnel loop is active.
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Status returns a coarse-grained runtime status.
func (s *Service) Status() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Detail returns the latest human-readable runtime detail.
func (s *Service) Detail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detail
}

// LastError returns the last background error string, if any.
func (s *Service) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

// RecentLogs returns a newline-delimited tail of recent client logs.
func (s *Service) RecentLogs() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.recentLogs, "\n")
}

// ActiveTunnelsJSON returns a JSON array of currently active server-side listeners.
func (s *Service) ActiveTunnelsJSON() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := json.Marshal(s.activeTunnelsLocked())
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func (s *Service) consumeLogEntry(entry protocol.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appendLogLocked(entry.Level, entry.Message, entry.Timestamp)

	lower := strings.ToLower(entry.Message)
	switch {
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
