package config

import (
	"os"
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

func TestGenerateWebJWTSecretCreatesIndependentSecret(t *testing.T) {
	cfg := &ServerConfig{}
	setDefaults(cfg)

	if cfg.Server.Web.JWTSecret != "" {
		t.Fatalf("expected empty JWT secret before generation, got %q", cfg.Server.Web.JWTSecret)
	}
	if !GenerateWebJWTSecret(cfg) {
		t.Fatal("expected GenerateWebJWTSecret to report a new secret")
	}
	if cfg.Server.Web.JWTSecret == "" {
		t.Fatal("expected JWT secret to be generated")
	}
	if cfg.Server.Web.JWTSecret == cfg.Server.Token {
		t.Fatal("expected web JWT secret to be isolated from tunnel token")
	}
}

func TestSaveServerConfigUsesRestrictedPermissions(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "server.yaml")
	cfg := &ServerConfig{}
	setDefaults(cfg)
	GenerateWebJWTSecret(cfg)

	if err := SaveServerConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveServerConfig returned error: %v", err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("unexpected file mode: got %o want 600", perm)
	}
}
