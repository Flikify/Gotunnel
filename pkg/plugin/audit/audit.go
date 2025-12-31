package audit

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventType 审计事件类型
type EventType string

const (
	EventPluginInstall   EventType = "plugin_install"
	EventPluginUninstall EventType = "plugin_uninstall"
	EventPluginStart     EventType = "plugin_start"
	EventPluginStop      EventType = "plugin_stop"
	EventPluginVerify    EventType = "plugin_verify"
	EventPluginReject    EventType = "plugin_reject"
	EventConfigChange    EventType = "config_change"
)

// Event 审计事件
type Event struct {
	Timestamp  time.Time         `json:"timestamp"`
	Type       EventType         `json:"type"`
	PluginName string            `json:"plugin_name,omitempty"`
	Version    string            `json:"version,omitempty"`
	ClientID   string            `json:"client_id,omitempty"`
	Success    bool              `json:"success"`
	Message    string            `json:"message,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

// Logger 审计日志记录器
type Logger struct {
	path    string
	file    *os.File
	mu      sync.Mutex
	enabled bool
}

var (
	defaultLogger *Logger
	loggerOnce    sync.Once
)

// NewLogger 创建审计日志记录器
func NewLogger(dataDir string) (*Logger, error) {
	path := filepath.Join(dataDir, "audit.log")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	return &Logger{path: path, file: file, enabled: true}, nil
}

// InitDefault 初始化默认日志记录器
func InitDefault(dataDir string) error {
	var err error
	loggerOnce.Do(func() {
		defaultLogger, err = NewLogger(dataDir)
	})
	return err
}

// Log 记录审计事件
func (l *Logger) Log(event Event) {
	if l == nil || !l.enabled {
		return
	}

	event.Timestamp = time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[Audit] Marshal error: %v", err)
		return
	}

	if _, err := l.file.Write(append(data, '\n')); err != nil {
		log.Printf("[Audit] Write error: %v", err)
	}
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// LogEvent 使用默认记录器记录事件
func LogEvent(event Event) {
	if defaultLogger != nil {
		defaultLogger.Log(event)
	}
}

// LogPluginInstall 记录插件安装事件
func LogPluginInstall(pluginName, version, clientID string, success bool, msg string) {
	LogEvent(Event{
		Type:       EventPluginInstall,
		PluginName: pluginName,
		Version:    version,
		ClientID:   clientID,
		Success:    success,
		Message:    msg,
	})
}

// LogPluginVerify 记录插件验证事件
func LogPluginVerify(pluginName, version string, success bool, msg string) {
	LogEvent(Event{
		Type:       EventPluginVerify,
		PluginName: pluginName,
		Version:    version,
		Success:    success,
		Message:    msg,
	})
}

// LogPluginReject 记录插件拒绝事件
func LogPluginReject(pluginName, version, reason string) {
	LogEvent(Event{
		Type:       EventPluginReject,
		PluginName: pluginName,
		Version:    version,
		Success:    false,
		Message:    reason,
	})
}

// LogWithDetails 记录带详情的事件
func LogWithDetails(eventType EventType, pluginName string, success bool, msg string, details map[string]string) {
	LogEvent(Event{
		Type:       eventType,
		PluginName: pluginName,
		Success:    success,
		Message:    msg,
		Details:    details,
	})
}
