package config

import (
	"path/filepath"
	"testing"
)

func TestLoadServerConfigSetsNewDefaults(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "server.yaml")

	cfg, err := LoadServerConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadServerConfig returned error: %v", err)
	}

	if cfg.Server.HeartbeatSec != 30 {
		t.Fatalf("expected default heartbeat_sec 30, got %d", cfg.Server.HeartbeatSec)
	}
	if cfg.Server.HeartbeatTimeout != 90 {
		t.Fatalf("expected default heartbeat_timeout 90, got %d", cfg.Server.HeartbeatTimeout)
	}
	if cfg.Server.MaxClientProxies != 0 {
		t.Fatalf("expected default max_client_proxies 0, got %d", cfg.Server.MaxClientProxies)
	}
	if cfg.Server.ClientResponseTimeoutSec != 15 {
		t.Fatalf("expected default client_response_timeout_sec 15, got %d", cfg.Server.ClientResponseTimeoutSec)
	}
}
