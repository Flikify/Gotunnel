package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/internal/server/updateapp"
)

func TestUpdateHandlerApplyClientMapsServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewUpdateHandler(&fakeUpdateService{applyClientErr: errors.New("push failed")})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/update/apply/client", strings.NewReader(`{"client_id":"client-1","download_url":"https://example.com/client.tar.gz"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ApplyClient(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected HTTP status: got %d want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateHandlerCheckClientDelegatesToService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewUpdateHandler(&fakeUpdateService{
		checkClientResult: &updateapp.Info{
			Available: true,
			Latest:    "v1.2.3",
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/update/check/client?os=linux&arch=amd64", nil)

	h.CheckClient(c)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected HTTP status: got %d want %d", w.Code, http.StatusOK)
	}
	if h.updates.(*fakeUpdateService).checkedOS != "linux" || h.updates.(*fakeUpdateService).checkedArch != "amd64" {
		t.Fatalf("expected service to receive query params, got os=%q arch=%q", h.updates.(*fakeUpdateService).checkedOS, h.updates.(*fakeUpdateService).checkedArch)
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Available bool   `json:"available"`
			Latest    string `json:"latest"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != CodeSuccess || !resp.Data.Available || resp.Data.Latest != "v1.2.3" {
		t.Fatalf("unexpected response payload: %+v", resp)
	}
}

type fakeUpdateService struct {
	checkServerResult *updateapp.Info
	checkClientResult *updateapp.Info
	applyServerErr    error
	applyClientErr    error
	checkedOS         string
	checkedArch       string
	clientID          string
	downloadURL       string
}

func (s *fakeUpdateService) CheckServer() (*updateapp.Info, error) {
	if s.checkServerResult == nil {
		return &updateapp.Info{}, nil
	}
	return s.checkServerResult, nil
}

func (s *fakeUpdateService) CheckClient(osName, arch string) (*updateapp.Info, error) {
	s.checkedOS = osName
	s.checkedArch = arch
	if s.checkClientResult == nil {
		return &updateapp.Info{}, nil
	}
	return s.checkClientResult, nil
}

func (s *fakeUpdateService) ApplyServer(downloadURL string, restart bool) error {
	s.downloadURL = downloadURL
	return s.applyServerErr
}

func (s *fakeUpdateService) ApplyClient(clientID, downloadURL string) error {
	s.clientID = clientID
	s.downloadURL = downloadURL
	return s.applyClientErr
}

var _ service.UpdateService = (*fakeUpdateService)(nil)
