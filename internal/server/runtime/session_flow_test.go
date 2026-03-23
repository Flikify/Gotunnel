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

type flowTestStore struct {
	clients       map[string]*db.Client
	rules         map[string][]domain.ProxyRule
	installTokens map[string]*db.InstallToken
	markedTokens  []string
}

func newFlowTestStore() *flowTestStore {
	return &flowTestStore{
		clients:       make(map[string]*db.Client),
		rules:         make(map[string][]domain.ProxyRule),
		installTokens: make(map[string]*db.InstallToken),
	}
}

func (s *flowTestStore) GetAllClients() ([]db.Client, error) {
	out := make([]db.Client, 0, len(s.clients))
	for _, client := range s.clients {
		out = append(out, *cloneClient(client))
	}
	return out, nil
}

func (s *flowTestStore) GetClient(id string) (*db.Client, error) {
	client, ok := s.clients[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return cloneClient(client), nil
}

func (s *flowTestStore) CreateClient(c *db.Client) error {
	s.clients[c.ID] = cloneClient(c)
	if _, ok := s.rules[c.ID]; !ok {
		s.rules[c.ID] = cloneProxyRules(c.Rules)
	}
	return nil
}

func (s *flowTestStore) UpdateClient(c *db.Client) error {
	s.clients[c.ID] = cloneClient(c)
	return nil
}

func (s *flowTestStore) DeleteClient(id string) error {
	delete(s.clients, id)
	delete(s.rules, id)
	return nil
}

func (s *flowTestStore) ClientExists(id string) (bool, error) {
	_, ok := s.clients[id]
	return ok, nil
}

func (s *flowTestStore) GetClientRules(id string) ([]domain.ProxyRule, error) {
	return cloneProxyRules(s.rules[id]), nil
}

func (s *flowTestStore) CreateInstallToken(token *db.InstallToken) error {
	copyToken := *token
	s.installTokens[token.Token] = &copyToken
	return nil
}

func (s *flowTestStore) GetInstallToken(token string) (*db.InstallToken, error) {
	installToken, ok := s.installTokens[token]
	if !ok {
		return nil, errors.New("not found")
	}
	copyToken := *installToken
	return &copyToken, nil
}

func (s *flowTestStore) MarkTokenUsed(token string) error {
	installToken, ok := s.installTokens[token]
	if !ok {
		return errors.New("not found")
	}
	installToken.Used = true
	s.markedTokens = append(s.markedTokens, token)
	return nil
}

func (s *flowTestStore) DeleteExpiredTokens(expireTime int64) error { return nil }
func (s *flowTestStore) Close() error                               { return nil }

func TestClientAdmissionRejectsInvalidToken(t *testing.T) {
	admission := newClientAdmission("server-token", newFlowTestStore())

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan error, 1)
	go func() {
		defer close(done)
		msg, err := protocol.NewMessage(protocol.MsgTypeAuth, protocol.AuthRequest{
			Token:    "bad-token",
			ClientID: "client-a",
		})
		if err != nil {
			done <- err
			return
		}
		done <- protocol.WriteMessage(clientConn, msg)
	}()

	_, err := admission.admit(serverConn)
	if err == nil {
		t.Fatal("expected invalid token error")
	}

	var rejection *admissionRejectionError
	if !errors.As(err, &rejection) {
		t.Fatalf("expected rejection error, got %T", err)
	}
	if rejection.message != "invalid token" {
		t.Fatalf("expected invalid token message, got %q", rejection.message)
	}
	if err := <-done; err != nil {
		t.Fatalf("client writer failed: %v", err)
	}
}

func TestClientAdmissionAcceptsInstallTokenAndBootstrapsClient(t *testing.T) {
	store := newFlowTestStore()
	store.rules["client-a"] = []domain.ProxyRule{{Name: "rule-a", RemotePort: 8080}}
	store.installTokens["install-token"] = &db.InstallToken{
		Token:     "install-token",
		CreatedAt: time.Now().Unix(),
	}

	admission := newClientAdmission("server-token", store)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan error, 1)
	go func() {
		defer close(done)
		msg, err := protocol.NewMessage(protocol.MsgTypeAuth, protocol.AuthRequest{
			Token:    "install-token",
			ClientID: "client-a",
			Name:     "alpha",
			OS:       "linux",
			Arch:     "amd64",
			Version:  "1.0.0",
		})
		if err != nil {
			done <- err
			return
		}
		done <- protocol.WriteMessage(clientConn, msg)
	}()

	admitted, err := admission.admit(serverConn)
	if err != nil {
		t.Fatalf("admit returned error: %v", err)
	}
	if admitted.ID != "client-a" || admitted.Name != "alpha" {
		t.Fatalf("unexpected admitted client: %+v", admitted)
	}
	if admitted.RemoteAddr == "" {
		t.Fatal("expected admitted client remote address")
	}
	if len(admitted.Rules) != 1 || admitted.Rules[0].Name != "rule-a" {
		t.Fatalf("unexpected admitted rules: %+v", admitted.Rules)
	}
	if len(store.markedTokens) != 1 || store.markedTokens[0] != "install-token" {
		t.Fatalf("expected install token to be marked used, got %+v", store.markedTokens)
	}
	client, ok := store.clients["client-a"]
	if !ok {
		t.Fatal("expected client record to be created")
	}
	if client.Nickname != "alpha" {
		t.Fatalf("expected client nickname to be persisted, got %q", client.Nickname)
	}
	if err := <-done; err != nil {
		t.Fatalf("client writer failed: %v", err)
	}
}

func TestSessionLifecycleRegistersAndCleansUpClient(t *testing.T) {
	store := newFlowTestStore()
	store.clients["client-a"] = &db.Client{ID: "client-a"}

	srv := NewServer(store, "127.0.0.1", 7000, "token", 30, 90)

	disabled := false
	admitted := &admittedClient{
		ID:         "client-a",
		Name:       "alpha",
		RemoteAddr: "127.0.0.1",
		OS:         "linux",
		Arch:       "amd64",
		Version:    "1.0.0",
		Rules: []domain.ProxyRule{
			{Name: "rule-a", RemotePort: 8080, Enabled: &disabled},
		},
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	clientSession, err := yamux.Client(clientConn, nil)
	if err != nil {
		t.Fatalf("create yamux client: %v", err)
	}
	defer clientSession.Close()

	ready := make(chan error, 1)
	allowClose := make(chan struct{})
	go func() {
		stream, err := clientSession.Accept()
		if err != nil {
			ready <- err
			return
		}
		defer stream.Close()

		msg, err := protocol.ReadMessage(stream)
		if err != nil {
			ready <- err
			return
		}
		if msg.Type != protocol.MsgTypeProxyConfig {
			ready <- errors.New("unexpected message type")
			return
		}

		ack, err := protocol.NewMessage(protocol.MsgTypeProxyReady, struct{}{})
		if err != nil {
			ready <- err
			return
		}
		if err := protocol.WriteMessage(stream, ack); err != nil {
			ready <- err
			return
		}

		ready <- nil
		<-allowClose
		_ = clientSession.Close()
	}()

	finished := make(chan struct{})
	go func() {
		srv.lifecycle.run(serverConn, admitted)
		close(finished)
	}()

	if err := <-ready; err != nil {
		t.Fatalf("client session setup failed: %v", err)
	}
	if !srv.IsClientOnline("client-a") {
		t.Fatal("expected client to be registered as online")
	}

	close(allowClose)

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("session lifecycle did not exit after client close")
	}

	if srv.IsClientOnline("client-a") {
		t.Fatal("expected client to be removed after session close")
	}
	client := store.clients["client-a"]
	if client.LastRemoteAddr != "127.0.0.1" || client.LastVersion != "1.0.0" {
		t.Fatalf("expected lifecycle to persist connection info, got %+v", client)
	}
}

func cloneClient(c *db.Client) *db.Client {
	if c == nil {
		return nil
	}

	copyClient := *c
	copyClient.Rules = cloneProxyRules(c.Rules)
	return &copyClient
}
