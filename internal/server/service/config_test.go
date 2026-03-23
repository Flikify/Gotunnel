package service

import (
	"path/filepath"
	"reflect"
	"testing"

	serverconfig "github.com/gotunnel/internal/server/config"
)

func TestConfigServicePersistDoesNotApplyRuntimeImmediately(t *testing.T) {
	cfg := &serverconfig.ServerConfig{
		Server: serverconfig.ServerSettings{
			HeartbeatSec:             30,
			HeartbeatTimeout:         90,
			MaxClientProxies:         1,
			ClientResponseTimeoutSec: 15,
		},
	}
	runtime := &fakeRuntimeConfig{}
	service := NewConfigService(cfg, filepath.Join(t.TempDir(), "server.yaml"), runtime)

	result, err := service.Persist(ConfigUpdate{
		Server: &ServerConfigUpdate{
			HeartbeatSec:             intPtr(45),
			MaxClientProxies:         intPtr(3),
			ClientResponseTimeoutSec: intPtr(20),
		},
	})
	if err != nil {
		t.Fatalf("persist config: %v", err)
	}
	if runtime.calls != 0 {
		t.Fatalf("expected runtime apply to be deferred, got %d calls", runtime.calls)
	}

	want := []string{"client_response_timeout_sec", "heartbeat_sec", "max_client_proxies"}
	if !reflect.DeepEqual(result.RuntimeApplyFields, want) {
		t.Fatalf("unexpected runtime fields: got %v want %v", result.RuntimeApplyFields, want)
	}
}

func TestConfigServiceApplyRuntimeConfigUsesPersistedSnapshot(t *testing.T) {
	cfg := &serverconfig.ServerConfig{
		Server: serverconfig.ServerSettings{
			HeartbeatSec:             30,
			HeartbeatTimeout:         90,
			MaxClientProxies:         1,
			ClientResponseTimeoutSec: 15,
		},
	}
	runtime := &fakeRuntimeConfig{}
	service := NewConfigService(cfg, filepath.Join(t.TempDir(), "server.yaml"), runtime)

	if _, err := service.Persist(ConfigUpdate{
		Server: &ServerConfigUpdate{
			HeartbeatSec:             intPtr(60),
			HeartbeatTimeout:         intPtr(120),
			MaxClientProxies:         intPtr(5),
			ClientResponseTimeoutSec: intPtr(25),
		},
	}); err != nil {
		t.Fatalf("persist config: %v", err)
	}

	applied := service.ApplyRuntimeConfig([]string{"heartbeat_sec", "heartbeat_timeout"})
	if runtime.calls != 1 {
		t.Fatalf("expected one runtime apply, got %d", runtime.calls)
	}
	if runtime.heartbeatSec != 60 || runtime.heartbeatTimeout != 120 || runtime.maxClientProxies != 5 || runtime.clientResponseTimeoutSec != 25 {
		t.Fatalf("runtime applied stale config: %+v", runtime)
	}
	if !reflect.DeepEqual(applied.AppliedRuntimeFields, []string{"heartbeat_sec", "heartbeat_timeout"}) {
		t.Fatalf("unexpected apply result: %+v", applied)
	}
}

type fakeRuntimeConfig struct {
	calls                    int
	heartbeatSec             int
	heartbeatTimeout         int
	maxClientProxies         int
	clientResponseTimeoutSec int
}

func (r *fakeRuntimeConfig) ApplyRuntimeConfig(heartbeatSec, heartbeatTimeoutSec, maxClientProxies, clientResponseTimeoutSec int) {
	r.calls++
	r.heartbeatSec = heartbeatSec
	r.heartbeatTimeout = heartbeatTimeoutSec
	r.maxClientProxies = maxClientProxies
	r.clientResponseTimeoutSec = clientResponseTimeoutSec
}

func intPtr(v int) *int {
	return &v
}
