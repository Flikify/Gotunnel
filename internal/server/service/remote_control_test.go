package service

import (
	"net"
	"testing"
	"time"

	serverruntime "github.com/gotunnel/internal/server/runtime"
	"github.com/gotunnel/pkg/observability"
	"github.com/gotunnel/pkg/protocol"
)

type fakeRemoteControlRuntime struct {
	online bool
}

func (f *fakeRemoteControlRuntime) IsClientOnline(clientID string) bool { return f.online }

func (f *fakeRemoteControlRuntime) OpenClientStream(clientID string) (net.Conn, error) {
	serverConn, clientConn := net.Pipe()
	go func() {
		defer clientConn.Close()

		msg, err := protocol.ReadMessage(clientConn)
		if err != nil {
			return
		}
		if msg.Type != protocol.MsgTypeRemoteControlStart {
			return
		}

		ready, _ := protocol.NewMessage(protocol.MsgTypeRemoteControlReady, protocol.RemoteControlReady{
			Width:           1280,
			Height:          720,
			FrameIntervalMS: 150,
		})
		_ = protocol.WriteMessage(clientConn, ready)

		for {
			if _, err := protocol.ReadMessage(clientConn); err != nil {
				return
			}
		}
	}()
	return serverConn, nil
}

func (f *fakeRemoteControlRuntime) ClientResponseTimeout() time.Duration { return time.Second }

func (f *fakeRemoteControlRuntime) LogSessions() *serverruntime.LogSessionManager {
	return serverruntime.NewLogSessionManager()
}

func (f *fakeRemoteControlRuntime) LocalDiagnosticStore() *observability.DiagnosticStore { return nil }

func TestRemoteControlServiceRejectsOfflineClient(t *testing.T) {
	service := NewRemoteControlService(&fakeRemoteControlRuntime{online: false})

	if _, err := service.OpenSession("client-1", protocol.RemoteControlStart{}); err != ErrClientNotOnline {
		t.Fatalf("unexpected error: got %v want %v", err, ErrClientNotOnline)
	}
}

func TestRemoteControlServiceRejectsSecondActiveSession(t *testing.T) {
	service := NewRemoteControlService(&fakeRemoteControlRuntime{online: true})

	first, err := service.OpenSession("client-1", protocol.RemoteControlStart{})
	if err != nil {
		t.Fatalf("OpenSession returned error: %v", err)
	}
	defer first.Stop("cleanup")

	if _, err := service.OpenSession("client-1", protocol.RemoteControlStart{}); err != ErrRemoteControlSessionActive {
		t.Fatalf("unexpected error: got %v want %v", err, ErrRemoteControlSessionActive)
	}
}
