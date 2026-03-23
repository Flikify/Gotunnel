package service

import (
	"errors"
	"testing"

	"github.com/gotunnel/internal/server/config"
	domain "github.com/gotunnel/internal/core/domain"
)

func TestClientServiceCreateClientPersistsRules(t *testing.T) {
	repo := &fakeClientRepository{}
	svc := NewClientService(repo, &fakeClientRuntime{}, &fakeConfigService{maxClientProxies: 2})

	rules := []domain.ProxyRule{{Name: "web", RemotePort: 8080}}
	if err := svc.CreateClient(CreateClientInput{ID: "client-1", Rules: rules}); err != nil {
		t.Fatalf("CreateClient returned error: %v", err)
	}
	if repo.client == nil {
		t.Fatal("expected client to be persisted")
	}
	if repo.client.ID != "client-1" {
		t.Fatalf("unexpected client id: %q", repo.client.ID)
	}
	if len(repo.client.Rules) != 1 || repo.client.Rules[0].Name != "web" {
		t.Fatalf("unexpected persisted rules: %+v", repo.client.Rules)
	}

	rules[0].Name = "changed"
	if repo.client.Rules[0].Name != "web" {
		t.Fatalf("expected rules to be copied, got %q", repo.client.Rules[0].Name)
	}
}

func TestClientServiceCreateClientRejectsTooManyRules(t *testing.T) {
	svc := NewClientService(
		&fakeClientRepository{},
		&fakeClientRuntime{},
		&fakeConfigService{maxClientProxies: 1},
	)

	err := svc.CreateClient(CreateClientInput{
		ID: "client-1",
		Rules: []domain.ProxyRule{
			{Name: "one", RemotePort: 1000},
			{Name: "two", RemotePort: 1001},
		},
	})
	if !errors.Is(err, ErrProxyRuleLimitExceeded) {
		t.Fatalf("expected ErrProxyRuleLimitExceeded, got %v", err)
	}
}

func TestClientServiceGetClientMergesRuntimeStatus(t *testing.T) {
	svc := NewClientService(
		&fakeClientRepository{
			client: &domain.Client{
				ID:             "client-1",
				Rules:          []domain.ProxyRule{{Name: "web", RemotePort: 8080}},
				LastRemoteAddr: "10.0.0.8",
				LastOS:         "linux",
				LastArch:       "amd64",
				LastVersion:    "1.0.0",
			},
		},
		&fakeClientRuntime{
			status: ClientRuntimeStatus{
				Online:     true,
				Name:       "host-a",
				LastPing:   "2026-03-23T10:00:00Z",
				RemoteAddr: "192.168.1.8",
				OS:         "darwin",
				Arch:       "arm64",
				Version:    "2.0.0",
			},
			statusOnline: true,
		},
		&fakeConfigService{},
	)

	client, err := svc.GetClient("client-1")
	if err != nil {
		t.Fatalf("GetClient returned error: %v", err)
	}
	if client.Nickname != "host-a" {
		t.Fatalf("expected runtime nickname fallback, got %q", client.Nickname)
	}
	if client.RemoteAddr != "192.168.1.8" || client.OS != "darwin" || client.Arch != "arm64" || client.Version != "2.0.0" {
		t.Fatalf("expected runtime status to win, got %+v", client)
	}
}

func TestClientServicePushConfigRequiresOnline(t *testing.T) {
	svc := NewClientService(
		&fakeClientRepository{},
		&fakeClientRuntime{},
		&fakeConfigService{},
	)

	err := svc.PushConfig("client-1")
	if !errors.Is(err, ErrClientNotOnline) {
		t.Fatalf("expected ErrClientNotOnline, got %v", err)
	}
}

type fakeClientRepository struct {
	allClients []domain.Client
	client     *domain.Client
	exists     bool
}

func (r *fakeClientRepository) GetAllClients() ([]domain.Client, error) {
	return r.allClients, nil
}

func (r *fakeClientRepository) GetClient(id string) (*domain.Client, error) {
	if r.client == nil {
		return nil, errors.New("not found")
	}
	return r.client, nil
}

func (r *fakeClientRepository) CreateClient(c *domain.Client) error {
	r.client = c
	return nil
}

func (r *fakeClientRepository) UpdateClient(c *domain.Client) error {
	r.client = c
	return nil
}

func (r *fakeClientRepository) DeleteClient(id string) error { return nil }

func (r *fakeClientRepository) ClientExists(id string) (bool, error) { return r.exists, nil }

func (r *fakeClientRepository) GetClientRules(id string) ([]domain.ProxyRule, error) {
	if r.client == nil {
		return nil, errors.New("not found")
	}
	return r.client.Rules, nil
}

type fakeClientRuntime struct {
	status       ClientRuntimeStatus
	statusOnline bool
	pushErr      error
}

func (r *fakeClientRuntime) IsClientOnline(clientID string) bool { return r.statusOnline }

func (r *fakeClientRuntime) GetClientStatus(clientID string) (ClientRuntimeStatus, bool) {
	return r.status, r.statusOnline
}

func (r *fakeClientRuntime) GetAllClientStatus() map[string]ClientRuntimeStatus {
	if !r.statusOnline {
		return map[string]ClientRuntimeStatus{}
	}
	return map[string]ClientRuntimeStatus{
		"client-1": r.status,
	}
}

func (r *fakeClientRuntime) PushConfigToClient(clientID string) error { return r.pushErr }

func (r *fakeClientRuntime) DisconnectClient(clientID string) error { return nil }

func (r *fakeClientRuntime) RestartClient(clientID string) error { return nil }

type fakeConfigService struct {
	maxClientProxies int
}

func (s *fakeConfigService) Snapshot() config.ServerConfig { return config.ServerConfig{} }

func (s *fakeConfigService) Persist(update ConfigUpdate) (PersistConfigResult, error) {
	return PersistConfigResult{}, nil
}

func (s *fakeConfigService) ApplyRuntimeConfig(fields []string) ConfigUpdateResult {
	return ConfigUpdateResult{AppliedRuntimeFields: fields}
}

func (s *fakeConfigService) MaxClientProxies() int { return s.maxClientProxies }
