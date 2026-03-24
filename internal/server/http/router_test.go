package httpapi

import (
	"net"
	"testing"
	"time"

	domain "github.com/gotunnel/internal/core/domain"
	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/http/handler"
	serverruntime "github.com/gotunnel/internal/server/runtime"
	"github.com/gotunnel/internal/server/service"
	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/auth"
	"github.com/gotunnel/pkg/observability"
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
		OperationalEvents: &fakeTrafficStore{},
		JWTAuth:           auth.NewJWTAuth("test-secret", 1),
		Username:          "admin",
		Password:          "admin",
	})

	want := map[string]bool{
		"GET /install.sh":                           false,
		"GET /api/clients/:id/logs":                 false,
		"GET /api/clients/:id/remote-control/ws":    false,
		"GET /api/updates/clients/latest":           false,
		"PUT /api/runtime/config":                   false,
		"POST /api/clients/:id/actions/push-config": false,
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

func (s *fakeServerRuntime) LogSessions() *serverruntime.LogSessionManager {
	return serverruntime.NewLogSessionManager()
}

func (s *fakeServerRuntime) LocalDiagnosticStore() *observability.DiagnosticStore { return nil }

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

func (s *fakeTrafficStore) AppendOperationalEvents(events []observability.OperationalEvent) error {
	return nil
}

func (s *fakeTrafficStore) ListOperationalEvents(filter observability.EventFilter) ([]observability.OperationalEvent, error) {
	return nil, nil
}

func (s *fakeTrafficStore) ListNodeHealth(limit int) ([]observability.NodeHealth, error) {
	return nil, nil
}

func (s *fakeTrafficStore) CountOperationalEventsSince(nodeID, eventCode string, since int64) (int, error) {
	return 0, nil
}
