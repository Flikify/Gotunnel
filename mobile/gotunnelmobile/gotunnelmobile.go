package gotunnelmobile

import (
	"context"
	"strings"
	"sync"

	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
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

	client    *tunnel.Client
	cancel    context.CancelFunc
	running   bool
	status    string
	lastError string
}

// NewService creates a mobile client service wrapper.
func NewService() *Service {
	return &Service{status: "stopped"}
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
	s.status = "running"
	s.lastError = ""
	s.mu.Unlock()

	go func() {
		err := client.RunContext(ctx)

		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		s.cancel = nil
		s.client = nil

		if err != nil {
			s.status = "error"
			s.lastError = err.Error()
			return
		}

		if s.status != "stopped" {
			s.status = "stopped"
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
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

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

// LastError returns the last background error string, if any.
func (s *Service) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}
