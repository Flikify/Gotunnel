package runtime

import (
	"sync"
	"time"

	domain "github.com/gotunnel/internal/core/domain"
	"github.com/hashicorp/yamux"
)

type clientSessionStatus struct {
	LastPing   string
	RemoteAddr string
	Name       string
	OS         string
	Arch       string
	Version    string
}

// ClientSession captures the live tunnel session and runtime metadata for a client.
type ClientSession struct {
	ID         string
	Name       string
	RemoteAddr string
	OS         string
	Arch       string
	Version    string
	Session    *yamux.Session
	Rules      []domain.ProxyRule
	LastPing   time.Time
	mu         sync.RWMutex
}

func newClientSession(session *yamux.Session, clientID, clientName, remoteAddr, clientOS, clientArch, clientVersion string, rules []domain.ProxyRule) *ClientSession {
	return &ClientSession{
		ID:         clientID,
		Name:       clientName,
		RemoteAddr: remoteAddr,
		OS:         clientOS,
		Arch:       clientArch,
		Version:    clientVersion,
		Session:    session,
		Rules:      cloneProxyRules(rules),
		LastPing:   time.Now(),
	}
}

func (cs *ClientSession) rulesSnapshot() []domain.ProxyRule {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cloneProxyRules(cs.Rules)
}

func (cs *ClientSession) setRules(rules []domain.ProxyRule) {
	cs.mu.Lock()
	cs.Rules = cloneProxyRules(rules)
	cs.mu.Unlock()
}

func (cs *ClientSession) updateLastPing(ts time.Time) {
	cs.mu.Lock()
	cs.LastPing = ts
	cs.mu.Unlock()
}

func (cs *ClientSession) heartbeatExpired(now time.Time, timeout time.Duration) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return now.Sub(cs.LastPing) > timeout
}

func (cs *ClientSession) statusSnapshot() clientSessionStatus {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return clientSessionStatus{
		LastPing:   cs.LastPing.Format(time.RFC3339),
		RemoteAddr: cs.RemoteAddr,
		Name:       cs.Name,
		OS:         cs.OS,
		Arch:       cs.Arch,
		Version:    cs.Version,
	}
}

func cloneProxyRules(rules []domain.ProxyRule) []domain.ProxyRule {
	if len(rules) == 0 {
		return nil
	}

	out := make([]domain.ProxyRule, len(rules))
	copy(out, rules)
	return out
}
