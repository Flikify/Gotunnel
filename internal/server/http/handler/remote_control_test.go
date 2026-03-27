package handler

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/gotunnel/internal/server/http/middleware"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/pkg/auth"
	"github.com/gotunnel/pkg/protocol"
)

type fakeRemoteControlService struct {
	openFn func(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error)
}

func (f *fakeRemoteControlService) OpenSession(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error) {
	return f.openFn(clientID, start)
}

func TestRemoteControlHandlerRequiresAuth(t *testing.T) {
	server, token := newRemoteControlTestServer(t, &fakeRemoteControlService{
		openFn: func(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error) {
			t.Fatal("service should not be called without auth")
			return nil, nil
		},
	})
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", 1) + "/api/clients/client-1/remote-control/ws"
	if _, _, err := websocket.DefaultDialer.Dial(url, nil); err == nil {
		t.Fatal("expected websocket auth failure")
	}

	_ = token
}

func TestRemoteControlHandlerSendsErrorEnvelopeOnOpenFailure(t *testing.T) {
	server, token := newRemoteControlTestServer(t, &fakeRemoteControlService{
		openFn: func(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error) {
			return nil, service.ErrClientNotOnline
		},
	})
	defer server.Close()

	ws := dialRemoteControlWS(t, server.URL, token)
	defer ws.Close()

	var msg map[string]any
	if err := ws.ReadJSON(&msg); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if msg["type"] != "error" {
		t.Fatalf("unexpected websocket message: %+v", msg)
	}
}

func TestRemoteControlHandlerPassesTuningParams(t *testing.T) {
	params := make(chan protocol.RemoteControlStart, 1)
	server, token := newRemoteControlTestServer(t, &fakeRemoteControlService{
		openFn: func(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error) {
			params <- start
			return nil, service.ErrClientNotOnline
		},
	})
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", 1) +
		"/api/clients/client-1/remote-control/ws?token=" + token +
		"&quality=45&max_side=1280&frame_interval_ms=80"
	headers := http.Header{}
	headers.Set("Origin", server.URL)
	ws, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer ws.Close()

	select {
	case got := <-params:
		if got.Quality != 45 || got.MaxSide != 1280 || got.FrameIntervalMS != 80 {
			t.Fatalf("unexpected start params: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected service to receive start params")
	}
}

func TestRemoteControlHandlerBridgesStopAndBrowserDisconnect(t *testing.T) {
	serverStream, clientStream := net.Pipe()
	defer clientStream.Close()

	server, token := newRemoteControlTestServer(t, &fakeRemoteControlService{
		openFn: func(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error) {
			return &service.RemoteControlSession{
				ClientID: clientID,
				Stream:   serverStream,
				Ready: protocol.RemoteControlReady{
					Width:           1280,
					Height:          720,
					FrameIntervalMS: 150,
				},
			}, nil
		},
	})
	defer server.Close()

	ws := dialRemoteControlWS(t, server.URL, token)

	var ready map[string]any
	if err := ws.ReadJSON(&ready); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if ready["type"] != "ready" {
		t.Fatalf("unexpected ready payload: %+v", ready)
	}

	stopMsg, err := protocol.NewMessage(protocol.MsgTypeRemoteControlStop, protocol.RemoteControlStop{Reason: "client stopped"})
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if err := protocol.WriteMessage(clientStream, stopMsg); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	var stopped map[string]any
	if err := ws.ReadJSON(&stopped); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if stopped["type"] != "stopped" {
		t.Fatalf("unexpected stopped payload: %+v", stopped)
	}

	ws.Close()

	serverStream2, clientStream2 := net.Pipe()
	defer clientStream2.Close()
	server2, token2 := newRemoteControlTestServer(t, &fakeRemoteControlService{
		openFn: func(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error) {
			return &service.RemoteControlSession{
				ClientID: clientID,
				Stream:   serverStream2,
				Ready: protocol.RemoteControlReady{
					Width:           1280,
					Height:          720,
					FrameIntervalMS: 150,
				},
			}, nil
		},
	})
	defer server2.Close()

	ws2 := dialRemoteControlWS(t, server2.URL, token2)
	defer ws2.Close()
	if err := ws2.ReadJSON(&ready); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}

	if err := ws2.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	clientStream2.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := protocol.ReadMessage(clientStream2)
	if err != nil {
		t.Fatalf("expected stop message after browser disconnect, got error: %v", err)
	}
	if msg.Type != protocol.MsgTypeRemoteControlStop {
		t.Fatalf("unexpected protocol message type: got %d want %d", msg.Type, protocol.MsgTypeRemoteControlStop)
	}
}

func TestRemoteControlHandlerRejectsCrossOriginRequests(t *testing.T) {
	server, token := newRemoteControlTestServer(t, &fakeRemoteControlService{
		openFn: func(clientID string, start protocol.RemoteControlStart) (*service.RemoteControlSession, error) {
			t.Fatal("service should not be called for rejected origin")
			return nil, nil
		},
	})
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", 1) + "/api/clients/client-1/remote-control/ws?token=" + token
	headers := http.Header{}
	headers.Set("Origin", "http://evil.example")

	if _, _, err := websocket.DefaultDialer.Dial(url, headers); err == nil {
		t.Fatal("expected websocket origin failure")
	}
}

func newRemoteControlTestServer(t *testing.T, remoteService service.RemoteControlService) (*httptest.Server, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()

	jwtAuth := auth.NewJWTAuth("test-secret", 1)
	engine.Use(middleware.JWTAuth(jwtAuth))
	engine.GET("/api/clients/:id/remote-control/ws", NewRemoteControlHandler(remoteService).Stream)

	token, err := jwtAuth.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	return httptest.NewServer(engine), token
}

func dialRemoteControlWS(t *testing.T, serverURL, token string) *websocket.Conn {
	t.Helper()

	url := strings.Replace(serverURL, "http", "ws", 1) + "/api/clients/client-1/remote-control/ws?token=" + token
	headers := http.Header{}
	headers.Set("Origin", serverURL)
	ws, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	return ws
}
