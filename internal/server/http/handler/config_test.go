package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/service"
)

func TestConfigHandlerUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &fakeConfigService{
		updateResult: service.ConfigUpdateResult{
			AppliedRuntimeFields: []string{"heartbeat_sec"},
		},
	}
	h := NewConfigHandler(svc)

	body := []byte(`{"server":{"heartbeat_sec":10,"heartbeat_timeout":20}}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/runtime/config", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	if !svc.updateCalled {
		t.Fatal("expected config persist to be called")
	}
	if !svc.applyCalled {
		t.Fatal("expected runtime apply to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected HTTP status: got %d want %d", w.Code, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Fatalf("unexpected response code: got %d want %d", resp.Code, CodeSuccess)
	}
	if resp.Message != "配置已保存并同步了可热生效项" {
		t.Fatalf("unexpected response message: got %q", resp.Message)
	}
}

func TestConfigHandlerUpdateReportsRestartRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &fakeConfigService{
		updateResult: service.ConfigUpdateResult{
			RestartRequiredFields: []string{"web.username"},
		},
	}
	h := NewConfigHandler(svc)

	body := []byte(`{"web":{"username":"ops"}}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/runtime/config", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Update(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Message != "配置已保存，部分变更需要重启后生效" {
		t.Fatalf("unexpected response message: got %q", resp.Message)
	}
}

type fakeConfigService struct {
	updateCalled bool
	applyCalled  bool
	updateResult service.ConfigUpdateResult
}

func (s *fakeConfigService) Snapshot() config.ServerConfig { return config.ServerConfig{} }

func (s *fakeConfigService) Persist(update service.ConfigUpdate) (service.PersistConfigResult, error) {
	s.updateCalled = true
	return service.PersistConfigResult{
		RuntimeApplyFields:    s.updateResult.AppliedRuntimeFields,
		RestartRequiredFields: s.updateResult.RestartRequiredFields,
	}, nil
}

func (s *fakeConfigService) ApplyRuntimeConfig(fields []string) service.ConfigUpdateResult {
	s.applyCalled = true
	return service.ConfigUpdateResult{AppliedRuntimeFields: fields}
}

func (s *fakeConfigService) MaxClientProxies() int { return 0 }
