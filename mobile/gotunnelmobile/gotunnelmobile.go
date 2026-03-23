package gotunnelmobile

import (
	"encoding/json"

	clientapp "github.com/gotunnel/internal/client/app"
	"github.com/gotunnel/internal/client/tunnel"
)

// Service exposes a gomobile-friendly wrapper around the shared client application service.
type Service struct {
	app *clientapp.Service
}

// NewService creates a mobile client service wrapper.
func NewService() *Service {
	return &Service{app: clientapp.NewService()}
}

// Configure stores the parameters used by Start.
func (s *Service) Configure(server, token, dataDir, clientName, clientID string, disableTLS bool) {
	features := tunnel.MobilePlatformFeatures()
	s.app.Configure(clientapp.Config{
		Server:     server,
		Token:      token,
		DataDir:    dataDir,
		ClientName: clientName,
		ClientID:   clientID,
		TLSEnabled: !disableTLS,
		Features:   &features,
	})
}

// Start launches the tunnel loop in the background.
func (s *Service) Start() string {
	return s.app.Start()
}

// Stop cancels the running tunnel loop.
func (s *Service) Stop() string {
	return s.app.Stop()
}

// Restart restarts the service with the stored configuration.
func (s *Service) Restart() string {
	return s.app.Restart()
}

// IsRunning reports whether the tunnel loop is active.
func (s *Service) IsRunning() bool {
	return s.app.Snapshot().IsRunning
}

// Status returns a coarse-grained runtime status.
func (s *Service) Status() string {
	return s.app.Snapshot().Status
}

// Detail returns the latest human-readable runtime detail.
func (s *Service) Detail() string {
	return s.app.Snapshot().Detail
}

// LastError returns the last background error string, if any.
func (s *Service) LastError() string {
	return s.app.Snapshot().LastError
}

// RecentLogs returns a newline-delimited tail of recent client logs.
func (s *Service) RecentLogs() string {
	return s.app.Snapshot().RecentLogs
}

// ActiveTunnelsJSON returns a JSON array of currently active server-side listeners.
func (s *Service) ActiveTunnelsJSON() string {
	payload, err := json.Marshal(s.app.Snapshot().ActiveTunnels)
	if err != nil {
		return "[]"
	}
	return string(payload)
}
