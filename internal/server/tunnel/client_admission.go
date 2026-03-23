package tunnel

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/domain"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/security"
)

type admittedClient struct {
	ID         string
	Name       string
	RemoteAddr string
	OS         string
	Arch       string
	Version    string
	Rules      []domain.ProxyRule
}

type admissionRejectionError struct {
	message string
}

func (e *admissionRejectionError) Error() string {
	return e.message
}

type clientAdmission struct {
	token       string
	clientStore db.ClientStore
}

func newClientAdmission(token string, clientStore db.ClientStore) *clientAdmission {
	return &clientAdmission{
		token:       token,
		clientStore: clientStore,
	}
}

func (a *clientAdmission) admit(conn net.Conn) (*admittedClient, error) {
	clientIP := conn.RemoteAddr().String()
	_ = conn.SetReadDeadline(time.Now().Add(authTimeout))
	defer conn.SetReadDeadline(time.Time{})

	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("read auth: %w", err)
	}
	if msg.Type != protocol.MsgTypeAuth {
		return nil, fmt.Errorf("expected auth, got %d", msg.Type)
	}

	var authReq protocol.AuthRequest
	if err := msg.ParsePayload(&authReq); err != nil {
		return nil, fmt.Errorf("parse auth: %w", err)
	}

	isInstallToken, err := a.validateToken(authReq.Token)
	if err != nil {
		security.LogInvalidToken(clientIP)
		return nil, err
	}

	if authReq.ClientID == "" || !isValidClientID(authReq.ClientID) {
		security.LogInvalidClientID(clientIP, authReq.ClientID)
		return nil, &admissionRejectionError{message: "invalid client id format"}
	}

	rules, err := a.ensureClientRecord(authReq)
	if err != nil {
		return nil, err
	}

	if isInstallToken {
		if tokenStore, ok := a.clientStore.(db.InstallTokenStore); ok {
			_ = tokenStore.MarkTokenUsed(authReq.Token)
		}
	}

	return &admittedClient{
		ID:         authReq.ClientID,
		Name:       authReq.Name,
		RemoteAddr: remoteHost(conn.RemoteAddr()),
		OS:         authReq.OS,
		Arch:       authReq.Arch,
		Version:    authReq.Version,
		Rules:      rules,
	}, nil
}

func (a *clientAdmission) validateToken(token string) (bool, error) {
	if token == a.token {
		return false, nil
	}

	tokenStore, ok := a.clientStore.(db.InstallTokenStore)
	if !ok {
		return false, &admissionRejectionError{message: "invalid token"}
	}

	installToken, err := tokenStore.GetInstallToken(token)
	if err != nil {
		return false, &admissionRejectionError{message: "invalid token"}
	}
	if installToken.Used || time.Now().Unix()-installToken.CreatedAt >= 3600 {
		return false, &admissionRejectionError{message: "invalid token"}
	}

	return true, nil
}

func (a *clientAdmission) ensureClientRecord(authReq protocol.AuthRequest) ([]domain.ProxyRule, error) {
	exists, err := a.clientStore.ClientExists(authReq.ClientID)
	if err != nil {
		return nil, fmt.Errorf("check client exists: %w", err)
	}

	if !exists {
		newClient := &db.Client{
			ID:       authReq.ClientID,
			Nickname: authReq.Name,
			Rules:    []domain.ProxyRule{},
		}
		if err := a.clientStore.CreateClient(newClient); err != nil {
			log.Printf("[Server] Create client error: %v", err)
			return nil, &admissionRejectionError{message: "failed to create client"}
		}
		log.Printf("[Server] New client registered: %s (%s)", authReq.ClientID, authReq.Name)
	} else if authReq.Name != "" {
		client, err := a.clientStore.GetClient(authReq.ClientID)
		if err == nil && client.Nickname == "" {
			client.Nickname = authReq.Name
			_ = a.clientStore.UpdateClient(client)
		}
	}

	rules, err := a.clientStore.GetClientRules(authReq.ClientID)
	if err != nil {
		return nil, fmt.Errorf("load client rules: %w", err)
	}
	return cloneProxyRules(rules), nil
}

func remoteHost(addr net.Addr) string {
	if addr == nil {
		return ""
	}

	host := addr.String()
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return parsedHost
	}
	return host
}
