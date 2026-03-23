package runtime

import (
	"fmt"
	"net"
	"sync"
)

type clientSessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*ClientSession
}

func newClientSessionRegistry() *clientSessionRegistry {
	return &clientSessionRegistry{
		sessions: make(map[string]*ClientSession),
	}
}

func (r *clientSessionRegistry) add(session *ClientSession) {
	r.mu.Lock()
	r.sessions[session.ID] = session
	r.mu.Unlock()
}

func (r *clientSessionRegistry) get(clientID string) (*ClientSession, bool) {
	r.mu.RLock()
	session, ok := r.sessions[clientID]
	r.mu.RUnlock()
	return session, ok
}

func (r *clientSessionRegistry) remove(clientID string) {
	r.mu.Lock()
	delete(r.sessions, clientID)
	r.mu.Unlock()
}

func (r *clientSessionRegistry) openStream(clientID string) (net.Conn, error) {
	session, ok := r.get(clientID)
	if !ok {
		return nil, fmt.Errorf("client %s not found or not online", clientID)
	}
	return session.Session.Open()
}

func (r *clientSessionRegistry) isOnline(clientID string) bool {
	r.mu.RLock()
	_, ok := r.sessions[clientID]
	r.mu.RUnlock()
	return ok
}

func (r *clientSessionRegistry) list() []*ClientSession {
	r.mu.RLock()
	sessions := make([]*ClientSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	r.mu.RUnlock()
	return sessions
}

func (r *clientSessionRegistry) disconnectAll() {
	for _, session := range r.list() {
		_ = session.Session.Close()
	}
}

func (r *clientSessionRegistry) status(clientID string) (clientSessionStatus, bool) {
	session, ok := r.get(clientID)
	if !ok {
		return clientSessionStatus{}, false
	}
	return session.statusSnapshot(), true
}

func (r *clientSessionRegistry) allStatus() map[string]clientSessionStatus {
	sessions := r.list()
	result := make(map[string]clientSessionStatus, len(sessions))
	for _, session := range sessions {
		result[session.ID] = session.statusSnapshot()
	}
	return result
}
