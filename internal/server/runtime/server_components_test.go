package runtime

import (
	"errors"
	"net"
	"testing"
	"time"

	domain "github.com/gotunnel/internal/core/domain"
	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/protocol"
	"github.com/hashicorp/yamux"
)

type testClientStore struct {
	rules map[string][]domain.ProxyRule
}

func (s *testClientStore) GetAllClients() ([]db.Client, error)             { return nil, nil }
func (s *testClientStore) GetClient(id string) (*db.Client, error)         { return &db.Client{ID: id}, nil }
func (s *testClientStore) CreateClient(c *db.Client) error                 { return nil }
func (s *testClientStore) UpdateClient(c *db.Client) error                 { return nil }
func (s *testClientStore) DeleteClient(id string) error                    { return nil }
func (s *testClientStore) ClientExists(id string) (bool, error)            { return true, nil }
func (s *testClientStore) CreateInstallToken(token *db.InstallToken) error { return nil }
func (s *testClientStore) GetInstallToken(token string) (*db.InstallToken, error) {
	return nil, errors.New("not found")
}
func (s *testClientStore) MarkTokenUsed(token string) error           { return nil }
func (s *testClientStore) DeleteExpiredTokens(expireTime int64) error { return nil }
func (s *testClientStore) Close() error                               { return nil }

func (s *testClientStore) GetClientRules(id string) ([]domain.ProxyRule, error) {
	return cloneProxyRules(s.rules[id]), nil
}

type stubListener struct{}

func (stubListener) Accept() (net.Conn, error) { return nil, errors.New("not implemented") }
func (stubListener) Close() error              { return nil }
func (stubListener) Addr() net.Addr            { return stubAddr("stub") }

type stubAddr string

func (a stubAddr) Network() string { return string(a) }
func (a stubAddr) String() string  { return string(a) }

func TestOpenClientStreamUsesSessionRegistry(t *testing.T) {
	srv := NewServer(nil, "127.0.0.1", 7000, "token", 30, 90)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	serverSession, err := yamux.Server(serverConn, nil)
	if err != nil {
		t.Fatalf("create yamux server: %v", err)
	}
	defer serverSession.Close()

	clientSession, err := yamux.Client(clientConn, nil)
	if err != nil {
		t.Fatalf("create yamux client: %v", err)
	}
	defer clientSession.Close()

	srv.registerClient(newClientSession(serverSession, "client-a", "alpha", "127.0.0.1", "linux", "amd64", "1.0.0", nil))

	accepted := make(chan error, 1)
	go func() {
		stream, err := clientSession.Accept()
		if err != nil {
			accepted <- err
			return
		}
		_ = stream.Close()
		accepted <- nil
	}()

	stream, err := srv.OpenClientStream("client-a")
	if err != nil {
		t.Fatalf("OpenClientStream returned error: %v", err)
	}
	_ = stream.Close()

	if err := <-accepted; err != nil {
		t.Fatalf("client session did not accept opened stream: %v", err)
	}
}

func TestPushConfigToClientUsesSessionRegistryAndUpdatesRules(t *testing.T) {
	disabled := false
	store := &testClientStore{
		rules: map[string][]domain.ProxyRule{
			"client-a": {
				{Name: "rule-a", RemotePort: 8080, Enabled: &disabled},
			},
		},
	}
	srv := NewServer(store, "127.0.0.1", 7000, "token", 30, 90)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	serverSession, err := yamux.Server(serverConn, nil)
	if err != nil {
		t.Fatalf("create yamux server: %v", err)
	}
	defer serverSession.Close()

	clientSession, err := yamux.Client(clientConn, nil)
	if err != nil {
		t.Fatalf("create yamux client: %v", err)
	}
	defer clientSession.Close()

	srv.registerClient(newClientSession(serverSession, "client-a", "alpha", "127.0.0.1", "linux", "amd64", "1.0.0", nil))

	done := make(chan error, 1)
	go func() {
		stream, err := clientSession.Accept()
		if err != nil {
			done <- err
			return
		}
		defer stream.Close()

		msg, err := protocol.ReadMessage(stream)
		if err != nil {
			done <- err
			return
		}
		if msg.Type != protocol.MsgTypeProxyConfig {
			done <- errors.New("unexpected message type")
			return
		}

		var cfg protocol.ProxyConfig
		if err := msg.ParsePayload(&cfg); err != nil {
			done <- err
			return
		}
		if len(cfg.Rules) != 1 || cfg.Rules[0].Name != "rule-a" {
			done <- errors.New("unexpected proxy config payload")
			return
		}

		ack, err := protocol.NewMessage(protocol.MsgTypeProxyReady, struct{}{})
		if err != nil {
			done <- err
			return
		}
		done <- protocol.WriteMessage(stream, ack)
	}()

	if err := srv.PushConfigToClient("client-a"); err != nil {
		t.Fatalf("PushConfigToClient returned error: %v", err)
	}

	rules := srv.sessions.sessions["client-a"].rulesSnapshot()
	if len(rules) != 1 || rules[0].Name != "rule-a" {
		t.Fatalf("expected updated rules in client session, got %+v", rules)
	}

	if err := <-done; err != nil {
		t.Fatalf("client side returned error: %v", err)
	}
}

func TestIsPortAvailableExcludesCurrentClientBindings(t *testing.T) {
	srv := NewServer(nil, "127.0.0.1", 7000, "token", 30, 90)
	port := 32001

	srv.proxies.bindTCPListener("client-a", port, stubListener{})

	if srv.IsPortAvailable(port, "client-b") {
		t.Fatalf("expected port %d to be unavailable for other clients", port)
	}
	if !srv.IsPortAvailable(port, "client-a") {
		t.Fatalf("expected port %d to be available for the owning client", port)
	}
}

func TestRestartClientSendsRestartRequest(t *testing.T) {
	srv := NewServer(&testClientStore{}, "127.0.0.1", 7000, "token", 30, 90)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	serverSession, err := yamux.Server(serverConn, nil)
	if err != nil {
		t.Fatalf("create yamux server: %v", err)
	}
	defer serverSession.Close()

	clientSession, err := yamux.Client(clientConn, nil)
	if err != nil {
		t.Fatalf("create yamux client: %v", err)
	}
	defer clientSession.Close()

	srv.registerClient(newClientSession(serverSession, "client-a", "alpha", "127.0.0.1", "linux", "amd64", "1.0.0", nil))

	done := make(chan error, 1)
	go func() {
		stream, err := clientSession.Accept()
		if err != nil {
			done <- err
			return
		}
		defer stream.Close()

		msg, err := protocol.ReadMessage(stream)
		if err != nil {
			done <- err
			return
		}
		if msg.Type != protocol.MsgTypeClientRestart {
			done <- errors.New("unexpected restart message type")
			return
		}

		var req protocol.ClientRestartRequest
		if err := msg.ParsePayload(&req); err != nil {
			done <- err
			return
		}
		if req.Reason != "server requested restart" {
			done <- errors.New("unexpected restart reason")
			return
		}
		done <- nil
	}()

	if err := srv.RestartClient("client-a"); err != nil {
		t.Fatalf("RestartClient returned error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("client side returned error: %v", err)
	}

	select {
	case <-clientSession.CloseChan():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected client session to be closed after restart")
	}
}

func TestSendUpdateToClientSendsDownloadRequest(t *testing.T) {
	srv := NewServer(&testClientStore{}, "127.0.0.1", 7000, "token", 30, 90)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	serverSession, err := yamux.Server(serverConn, nil)
	if err != nil {
		t.Fatalf("create yamux server: %v", err)
	}
	defer serverSession.Close()

	clientSession, err := yamux.Client(clientConn, nil)
	if err != nil {
		t.Fatalf("create yamux client: %v", err)
	}
	defer clientSession.Close()

	srv.registerClient(newClientSession(serverSession, "client-a", "alpha", "127.0.0.1", "linux", "amd64", "1.0.0", nil))

	done := make(chan error, 1)
	go func() {
		stream, err := clientSession.Accept()
		if err != nil {
			done <- err
			return
		}
		defer stream.Close()

		msg, err := protocol.ReadMessage(stream)
		if err != nil {
			done <- err
			return
		}
		if msg.Type != protocol.MsgTypeUpdateDownload {
			done <- errors.New("unexpected update message type")
			return
		}

		var req protocol.UpdateDownloadRequest
		if err := msg.ParsePayload(&req); err != nil {
			done <- err
			return
		}
		if req.DownloadURL != "https://example.com/client.tar.gz" {
			done <- errors.New("unexpected update url")
			return
		}
		done <- nil
	}()

	if err := srv.SendUpdateToClient("client-a", "https://example.com/client.tar.gz"); err != nil {
		t.Fatalf("SendUpdateToClient returned error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("client side returned error: %v", err)
	}
}

func TestListenerRuntimeRejectsWhenConnectionLimitReached(t *testing.T) {
	runtime := newListenerRuntime(1)
	runtime.connSem <- struct{}{}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	called := false
	done := make(chan struct{})
	go func() {
		runtime.handleAcceptedConn(serverConn, func(net.Conn) {
			called = true
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("listener runtime did not return after rejecting connection")
	}

	if called {
		t.Fatal("handler should not run when connection limit is reached")
	}
}
