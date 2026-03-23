package tunnel

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gotunnel/internal/server/domain"
	"github.com/gotunnel/pkg/protocol"
)

func TestValidateProxyRuleLimit(t *testing.T) {
	srv := NewServer(nil, "127.0.0.1", 7000, "token", 30, 90)
	srv.ApplyRuntimeConfig(30, 90, 1, 2)

	err := srv.validateProxyRuleLimit([]domain.ProxyRule{
		{Name: "one", RemotePort: 1000},
		{Name: "two", RemotePort: 1001},
	})
	if err == nil {
		t.Fatal("expected proxy rule limit validation error")
	}
}

func TestRequestProxyOpenSuccess(t *testing.T) {
	srv := NewServer(nil, "127.0.0.1", 7000, "token", 30, 90)
	srv.ApplyRuntimeConfig(30, 90, 0, 1)

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	done := make(chan error, 1)
	go func() {
		defer close(done)
		msg, err := protocol.ReadMessage(clientSide)
		if err != nil {
			done <- err
			return
		}
		if msg.Type != protocol.MsgTypeNewProxy {
			done <- errors.New("unexpected message type")
			return
		}

		resp, err := protocol.NewMessage(protocol.MsgTypeProxyResult, protocol.ProxyConnectResult{Success: true})
		if err != nil {
			done <- err
			return
		}
		done <- protocol.WriteMessage(clientSide, resp)
	}()

	if err := srv.requestProxyOpen(serverSide, 8080); err != nil {
		t.Fatalf("requestProxyOpen returned error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("client side returned error: %v", err)
	}
}

func TestRequestProxyOpenTimeout(t *testing.T) {
	srv := NewServer(nil, "127.0.0.1", 7000, "token", 30, 90)
	srv.ApplyRuntimeConfig(30, 90, 0, 1)

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	go func() {
		_, _ = protocol.ReadMessage(clientSide)
		time.Sleep(1500 * time.Millisecond)
	}()

	if err := srv.requestProxyOpen(serverSide, 8080); err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
