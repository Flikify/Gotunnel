package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	domain "github.com/gotunnel/internal/core/domain"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/pkg/protocol"
)

func TestClientHandlerCreateMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewClientHandler(&fakeClientService{createErr: service.ErrClientAlreadyExists}, &fakeRemoteOpsService{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/clients", strings.NewReader(`{"id":"client-1"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("unexpected HTTP status: got %d want %d", w.Code, http.StatusConflict)
	}
}

func TestClientHandlerPushConfigMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewClientHandler(&fakeClientService{pushErr: service.ErrClientNotOnline}, &fakeRemoteOpsService{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "client-1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/clients/client-1/actions/push-config", nil)

	h.PushConfig(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected HTTP status: got %d want %d", w.Code, http.StatusBadRequest)
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != CodeClientNotOnline {
		t.Fatalf("unexpected response code: got %d want %d", resp.Code, CodeClientNotOnline)
	}
	if resp.Message != "client not online" {
		t.Fatalf("unexpected response message: got %q", resp.Message)
	}
}

func TestClientHandlerGetMapsDomainRulesToDTO(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewClientHandler(&fakeClientService{
		detail: &service.ClientDetail{
			ID:       "client-1",
			Nickname: "demo",
			Rules: []domain.ProxyRule{
				{Name: "web", Type: "tcp", RemotePort: 8080},
			},
			Online: true,
		},
	}, &fakeRemoteOpsService{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "client-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/clients/client-1", nil)

	h.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected HTTP status: got %d want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int               `json:"code"`
		Data dtoClientResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Fatalf("unexpected response code: got %d want %d", resp.Code, CodeSuccess)
	}
	if len(resp.Data.Rules) != 1 || resp.Data.Rules[0].RemotePort != 8080 {
		t.Fatalf("unexpected rules payload: %+v", resp.Data.Rules)
	}
}

type dtoClientResponse struct {
	ID    string         `json:"id"`
	Rules []dtoProxyRule `json:"rules"`
}

type dtoProxyRule struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	RemotePort int    `json:"remote_port"`
}

type fakeClientService struct {
	createErr error
	pushErr   error
	detail    *service.ClientDetail
}

func (s *fakeClientService) ListClients() ([]service.ClientListItem, error) { return nil, nil }

func (s *fakeClientService) CreateClient(input service.CreateClientInput) error { return s.createErr }

func (s *fakeClientService) GetClient(id string) (*service.ClientDetail, error) {
	if s.detail == nil {
		return nil, service.ErrClientNotFound
	}
	return s.detail, nil
}

func (s *fakeClientService) UpdateClient(id string, input service.UpdateClientInput) error {
	return nil
}

func (s *fakeClientService) DeleteClient(id string) error { return nil }

func (s *fakeClientService) PushConfig(clientID string) error { return s.pushErr }

func (s *fakeClientService) DisconnectClient(clientID string) error { return nil }

func (s *fakeClientService) RestartClient(clientID string) error { return nil }

type fakeRemoteOpsService struct{}

func (s *fakeRemoteOpsService) IsClientOnline(clientID string) bool { return true }

func (s *fakeRemoteOpsService) StartClientLogStream(clientID, sessionID string, lines int, follow bool, level string) (<-chan protocol.LogEntry, error) {
	return nil, errors.New("unused")
}

func (s *fakeRemoteOpsService) StopClientLogStream(sessionID string) {}

func (s *fakeRemoteOpsService) GetClientSystemStats(clientID string) (*protocol.SystemStatsResponse, error) {
	return nil, errors.New("unused")
}

func (s *fakeRemoteOpsService) GetClientScreenshot(clientID string, quality int) (*protocol.ScreenshotResponse, error) {
	return nil, errors.New("unused")
}
