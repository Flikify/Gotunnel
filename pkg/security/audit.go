package security

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// EventType 安全事件类型
type EventType string

const (
	EventAuthSuccess    EventType = "AUTH_SUCCESS"
	EventAuthFailed     EventType = "AUTH_FAILED"
	EventInvalidToken   EventType = "INVALID_TOKEN"
	EventInvalidClientID EventType = "INVALID_CLIENT_ID"
	EventConnRejected   EventType = "CONN_REJECTED"
	EventConnLimit      EventType = "CONN_LIMIT"
	EventWebLoginOK     EventType = "WEB_LOGIN_OK"
	EventWebLoginFail   EventType = "WEB_LOGIN_FAIL"
)

// AuditEvent 审计事件
type AuditEvent struct {
	Time      time.Time
	Type      EventType
	ClientIP  string
	ClientID  string
	Message   string
}

// AuditLogger 审计日志记录器
type AuditLogger struct {
	mu     sync.Mutex
	events []AuditEvent
	maxLen int
}

var (
	defaultLogger *AuditLogger
	once          sync.Once
)

// GetAuditLogger 获取默认审计日志记录器
func GetAuditLogger() *AuditLogger {
	once.Do(func() {
		defaultLogger = &AuditLogger{
			events: make([]AuditEvent, 0, 1000),
			maxLen: 1000,
		}
	})
	return defaultLogger
}

// Log 记录安全事件
func (l *AuditLogger) Log(eventType EventType, clientIP, clientID, message string) {
	event := AuditEvent{
		Time:     time.Now(),
		Type:     eventType,
		ClientIP: clientIP,
		ClientID: clientID,
		Message:  message,
	}

	// 输出到标准日志
	log.Printf("[Security] %s | IP=%s | ID=%s | %s",
		eventType, clientIP, clientID, message)

	// 保存到内存（用于审计查询）
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, event)
	if len(l.events) > l.maxLen {
		l.events = l.events[1:]
	}
}

// GetRecentEvents 获取最近的安全事件
func (l *AuditLogger) GetRecentEvents(limit int) []AuditEvent {
	l.mu.Lock()
	defer l.mu.Unlock()

	if limit <= 0 || limit > len(l.events) {
		limit = len(l.events)
	}

	start := len(l.events) - limit
	result := make([]AuditEvent, limit)
	copy(result, l.events[start:])
	return result
}

// 便捷函数
func LogAuthSuccess(clientIP, clientID string) {
	GetAuditLogger().Log(EventAuthSuccess, clientIP, clientID, "authentication successful")
}

func LogAuthFailed(clientIP, clientID, reason string) {
	GetAuditLogger().Log(EventAuthFailed, clientIP, clientID,
		fmt.Sprintf("authentication failed: %s", reason))
}

func LogInvalidToken(clientIP string) {
	GetAuditLogger().Log(EventInvalidToken, clientIP, "", "invalid token provided")
}

func LogInvalidClientID(clientIP, clientID string) {
	GetAuditLogger().Log(EventInvalidClientID, clientIP, clientID, "invalid client ID format")
}

func LogConnRejected(clientIP, reason string) {
	GetAuditLogger().Log(EventConnRejected, clientIP, "", reason)
}

func LogWebLogin(clientIP, username string, success bool) {
	if success {
		GetAuditLogger().Log(EventWebLoginOK, clientIP, username, "web login successful")
	} else {
		GetAuditLogger().Log(EventWebLoginFail, clientIP, username, "web login failed")
	}
}
