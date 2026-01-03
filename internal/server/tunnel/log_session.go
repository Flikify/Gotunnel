package tunnel

import (
	"net"
	"sync"

	"github.com/gotunnel/pkg/protocol"
)

// LogSessionManager 管理所有活跃的日志会话
type LogSessionManager struct {
	sessions map[string]*LogSession
	mu       sync.RWMutex
}

// LogSession 日志流会话
type LogSession struct {
	ID        string
	ClientID  string
	Stream    net.Conn
	listeners []chan protocol.LogEntry
	mu        sync.Mutex
	closed    bool
}

// NewLogSessionManager 创建日志会话管理器
func NewLogSessionManager() *LogSessionManager {
	return &LogSessionManager{
		sessions: make(map[string]*LogSession),
	}
}

// CreateSession 创建日志会话
func (m *LogSessionManager) CreateSession(clientID, sessionID string, stream net.Conn) *LogSession {
	session := &LogSession{
		ID:        sessionID,
		ClientID:  clientID,
		Stream:    stream,
		listeners: make([]chan protocol.LogEntry, 0),
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	return session
}

// GetSession 获取会话
func (m *LogSessionManager) GetSession(sessionID string) *LogSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// RemoveSession 移除会话
func (m *LogSessionManager) RemoveSession(sessionID string) {
	m.mu.Lock()
	if session, ok := m.sessions[sessionID]; ok {
		session.Close()
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()
}

// GetSessionsByClient 获取客户端的所有会话
func (m *LogSessionManager) GetSessionsByClient(clientID string) []*LogSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*LogSession
	for _, session := range m.sessions {
		if session.ClientID == clientID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// CleanupClientSessions 清理客户端的所有会话
func (m *LogSessionManager) CleanupClientSessions(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.sessions {
		if session.ClientID == clientID {
			session.Close()
			delete(m.sessions, id)
		}
	}
}

// AddListener 添加监听器
func (s *LogSession) AddListener() <-chan protocol.LogEntry {
	ch := make(chan protocol.LogEntry, 100)
	s.mu.Lock()
	s.listeners = append(s.listeners, ch)
	s.mu.Unlock()
	return ch
}

// RemoveListener 移除监听器
func (s *LogSession) RemoveListener(ch <-chan protocol.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, listener := range s.listeners {
		if listener == ch {
			close(listener)
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			break
		}
	}
}

// Broadcast 广播日志条目到所有监听器
func (s *LogSession) Broadcast(entry protocol.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.listeners {
		select {
		case ch <- entry:
		default:
			// 监听器太慢，丢弃日志
		}
	}
}

// Close 关闭会话
func (s *LogSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	for _, ch := range s.listeners {
		close(ch)
	}
	s.listeners = nil

	if s.Stream != nil {
		s.Stream.Close()
	}
}

// IsClosed 检查会话是否已关闭
func (s *LogSession) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
