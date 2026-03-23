package tunnel

import (
	"container/ring"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotunnel/pkg/protocol"
)

const (
	maxBufferSize  = 1000            // 环形缓冲区最大条目数
	logFilePattern = "client.%s.log" // 日志文件名模式
)

// LogLevel 日志级别
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger 客户端日志收集器
type Logger struct {
	dataDir     string
	buffer      *ring.Ring
	bufferMu    sync.RWMutex
	file        *os.File
	fileMu      sync.Mutex
	fileDate    string
	subscribers map[string]chan protocol.LogEntry
	subMu       sync.RWMutex
	observers   map[int]func(protocol.LogEntry)
	observerMu  sync.RWMutex
	nextObsID   int
}

// NewLogger 创建新的日志收集器
func NewLogger(dataDir string) (*Logger, error) {
	l := &Logger{
		dataDir:     dataDir,
		buffer:      ring.New(maxBufferSize),
		subscribers: make(map[string]chan protocol.LogEntry),
		observers:   make(map[int]func(protocol.LogEntry)),
	}

	// 确保日志目录存在
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	return l, nil
}

// Printf 记录日志 (兼容 log.Printf)
func (l *Logger) Printf(format string, args ...interface{}) {
	l.log(LevelInfo, "client", format, args...)
}

// Infof 记录信息日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(LevelInfo, "client", format, args...)
}

// Warnf 记录警告日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(LevelWarn, "client", format, args...)
}

// Errorf 记录错误日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(LevelError, "client", format, args...)
}

// Debugf 记录调试日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(LevelDebug, "client", format, args...)
}

// PluginLog 记录插件日志
func (l *Logger) PluginLog(pluginName, level, format string, args ...interface{}) {
	var lvl LogLevel
	switch level {
	case "debug":
		lvl = LevelDebug
	case "warn":
		lvl = LevelWarn
	case "error":
		lvl = LevelError
	default:
		lvl = LevelInfo
	}
	l.log(lvl, "plugin:"+pluginName, format, args...)
}

func (l *Logger) log(level LogLevel, source, format string, args ...interface{}) {
	entry := protocol.LogEntry{
		Timestamp: time.Now().UnixMilli(),
		Level:     levelToString(level),
		Message:   fmt.Sprintf(format, args...),
		Source:    source,
	}

	// 注意：不在这里输出到标准输出，因为调用方（logf/logErrorf/logWarnf）已经调用了 log.Print
	// 这里只负责：缓冲区存储、文件写入、订阅者通知

	// 添加到环形缓冲区
	l.bufferMu.Lock()
	l.buffer.Value = entry
	l.buffer = l.buffer.Next()
	l.bufferMu.Unlock()

	// 写入文件
	l.writeToFile(entry)

	// 通知订阅者
	l.notifySubscribers(entry)
	l.notifyObservers(entry)
}

// Subscribe 订阅日志流
func (l *Logger) Subscribe(sessionID string) <-chan protocol.LogEntry {
	ch := make(chan protocol.LogEntry, 100)
	l.subMu.Lock()
	l.subscribers[sessionID] = ch
	l.subMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (l *Logger) Unsubscribe(sessionID string) {
	l.subMu.Lock()
	if ch, ok := l.subscribers[sessionID]; ok {
		close(ch)
		delete(l.subscribers, sessionID)
	}
	l.subMu.Unlock()
}

// AddObserver registers an in-process observer for every new log entry.
func (l *Logger) AddObserver(fn func(protocol.LogEntry)) func() {
	if fn == nil {
		return func() {}
	}

	l.observerMu.Lock()
	id := l.nextObsID
	l.nextObsID++
	l.observers[id] = fn
	l.observerMu.Unlock()

	return func() {
		l.observerMu.Lock()
		delete(l.observers, id)
		l.observerMu.Unlock()
	}
}

// GetRecentLogs 获取最近的日志
func (l *Logger) GetRecentLogs(lines int, level string) []protocol.LogEntry {
	l.bufferMu.RLock()
	defer l.bufferMu.RUnlock()

	var entries []protocol.LogEntry
	l.buffer.Do(func(v interface{}) {
		if v == nil {
			return
		}
		entry := v.(protocol.LogEntry)
		// 应用级别过滤
		if level != "" && entry.Level != level {
			return
		}
		entries = append(entries, entry)
	})

	// 如果指定了行数，返回最后 N 行
	if lines > 0 && len(entries) > lines {
		entries = entries[len(entries)-lines:]
	}

	return entries
}

// Close 关闭日志收集器
func (l *Logger) Close() {
	l.fileMu.Lock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	l.fileMu.Unlock()

	l.subMu.Lock()
	for _, ch := range l.subscribers {
		close(ch)
	}
	l.subscribers = make(map[string]chan protocol.LogEntry)
	l.subMu.Unlock()

	l.observerMu.Lock()
	l.observers = make(map[int]func(protocol.LogEntry))
	l.observerMu.Unlock()
}

func (l *Logger) writeToFile(entry protocol.LogEntry) {
	l.fileMu.Lock()
	defer l.fileMu.Unlock()

	today := time.Now().Format("2006-01-02")
	if l.fileDate != today {
		if l.file != nil {
			l.file.Close()
		}

		logPath := filepath.Join(l.dataDir, "logs", fmt.Sprintf(logFilePattern, today))
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		l.file = f
		l.fileDate = today
	}

	if l.file != nil {
		fmt.Fprintf(l.file, "%d|%s|%s|%s\n",
			entry.Timestamp, entry.Level, entry.Source, entry.Message)
	}
}

func (l *Logger) notifySubscribers(entry protocol.LogEntry) {
	l.subMu.RLock()
	defer l.subMu.RUnlock()

	for _, ch := range l.subscribers {
		select {
		case ch <- entry:
		default:
			// 订阅者太慢，丢弃日志
		}
	}
}

func (l *Logger) notifyObservers(entry protocol.LogEntry) {
	l.observerMu.RLock()
	observers := make([]func(protocol.LogEntry), 0, len(l.observers))
	for _, fn := range l.observers {
		observers = append(observers, fn)
	}
	l.observerMu.RUnlock()

	for _, fn := range observers {
		fn(entry)
	}
}

func levelToString(level LogLevel) string {
	switch level {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}
