package router

import (
	"net"
	"testing"
	"time"

	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/domain"
	"github.com/gotunnel/internal/server/router/handler"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/internal/server/tunnel"
	"github.com/gotunnel/pkg/auth"
)

var _ db.ClientStore = (*fakeClientStore)(nil)
var _ handler.ServerInterface = (*fakeServerRuntime)(nil)

func TestSetupRoutesRegistersCoreEndpoints(t *testing.T) {
	r := New()
	store := &fakeClientStore{}
	r.SetupRoutes(Dependencies{
		ClientStore:       store,
		InstallTokenStore: store,
		ServerRuntime:     &fakeServerRuntime{},
		ConfigService:     &fakeConfigService{},
		TrafficStore:      &fakeTrafficStore{},
		JWTAuth:           auth.NewJWTAuth("test-secret", 1),
		Username:          "admin",
		Password:          "admin",
	})

	want := map[string]bool{
		"GET /install.sh":              false,
		"GET /api/client/:id/logs":     false,
		"GET /api/update/check/client": false,
		"PUT /api/config":              false,
	}

	for _, route := range r.Engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}

	for route, found := range want {
		if !found {
			t.Fatalf("expected route %q to be registered", route)
		}
	}
}

type fakeClientStore struct{}

func (s *fakeClientStore) GetAllClients() ([]db.Client, error) { return nil, nil }

func (s *fakeClientStore) GetClient(id string) (*db.Client, error) { return nil, nil }

func (s *fakeClientStore) CreateClient(c *db.Client) error { return nil }

func (s *fakeClientStore) UpdateClient(c *db.Client) error { return nil }

func (s *fakeClientStore) DeleteClient(id string) error { return nil }

func (s *fakeClientStore) ClientExists(id string) (bool, error) { return false, nil }

func (s *fakeClientStore) GetClientRules(id string) ([]domain.ProxyRule, error) { return nil, nil }

func (s *fakeClientStore) Close() error { return nil }

func (s *fakeClientStore) CreateInstallToken(token *db.InstallToken) error { return nil }

func (s *fakeClientStore) GetInstallToken(token string) (*db.InstallToken, error) { return nil, nil }

func (s *fakeClientStore) MarkTokenUsed(token string) error { return nil }

func (s *fakeClientStore) DeleteExpiredTokens(expireTime int64) error { return nil }

type fakeServerRuntime struct{}

func (s *fakeServerRuntime) IsClientOnline(clientID string) bool { return false }

func (s *fakeServerRuntime) GetClientStatus(clientID string) (bool, string, string, string, string, string, string) {
	return false, "", "", "", "", "", ""
}

func (s *fakeServerRuntime) GetAllClientStatus() map[string]struct {
	Online     bool
	LastPing   string
	RemoteAddr string
	Name       string
	OS         string
	Arch       string
	Version    string
} {
	return map[string]struct {
		Online     bool
		LastPing   string
		RemoteAddr string
		Name       string
		OS         string
		Arch       string
		Version    string
	}{}
}

func (s *fakeServerRuntime) PushConfigToClient(clientID string) error { return nil }

func (s *fakeServerRuntime) DisconnectClient(clientID string) error { return nil }

func (s *fakeServerRuntime) RestartClient(clientID string) error { return nil }

func (s *fakeServerRuntime) SendUpdateToClient(clientID, downloadURL string) error { return nil }

func (s *fakeServerRuntime) GetBindAddr() string { return "127.0.0.1" }

func (s *fakeServerRuntime) GetBindPort() int { return 7000 }

func (s *fakeServerRuntime) ApplyRuntimeConfig(heartbeatSec, heartbeatTimeoutSec, maxClientProxies, clientResponseTimeoutSec int) {
}

func (s *fakeServerRuntime) IsPortAvailable(port int, excludeClientID string) bool { return true }

func (s *fakeServerRuntime) OpenClientStream(clientID string) (net.Conn, error) { return nil, nil }

func (s *fakeServerRuntime) ClientResponseTimeout() time.Duration { return time.Second }

func (s *fakeServerRuntime) LogSessions() *tunnel.LogSessionManager {
	return tunnel.NewLogSessionManager()
}

type fakeConfigService struct{}

func (s *fakeConfigService) Snapshot() config.ServerConfig { return config.ServerConfig{} }

func (s *fakeConfigService) Persist(update service.ConfigUpdate) (service.PersistConfigResult, error) {
	return service.PersistConfigResult{}, nil
}

func (s *fakeConfigService) ApplyRuntimeConfig(fields []string) service.ConfigUpdateResult {
	return service.ConfigUpdateResult{AppliedRuntimeFields: fields}
}

func (s *fakeConfigService) MaxClientProxies() int { return 0 }

type fakeTrafficStore struct{}

func (s *fakeTrafficStore) AddTraffic(inbound, outbound int64) error { return nil }

func (s *fakeTrafficStore) GetTotalTraffic() (int64, int64, error) { return 0, 0, nil }

func (s *fakeTrafficStore) Get24HourTraffic() (int64, int64, error) { return 0, 0, nil }

func (s *fakeTrafficStore) GetHourlyTraffic(hours int) ([]db.TrafficRecord, error) { return nil, nil }
