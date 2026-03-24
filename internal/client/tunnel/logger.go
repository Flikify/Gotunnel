package tunnel

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotunnel/pkg/observability"
	"github.com/gotunnel/pkg/protocol"
)

type Logger struct {
	store      *observability.DiagnosticStore
	observerMu sync.RWMutex
	observers  map[int]func(observedRecord)
	nextObsID  int
}

type observedRecord struct {
	Diagnostic observability.DiagnosticRecord
	Legacy     protocol.LogEntry
}

func NewLogger(dataDir, nodeID string) (*Logger, error) {
	store, err := observability.NewDiagnosticStore(observability.StoreOptions{
		RootDir:       filepath.Join(dataDir, "diagnostics"),
		RetentionDays: 7,
		NodeID:        nodeID,
		NodeRole:      observability.NodeRoleClient,
	})
	if err != nil {
		return nil, err
	}
	return &Logger{
		store:     store,
		observers: make(map[int]func(observedRecord)),
	}, nil
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.Record(observability.LevelInfo, "client", observability.EventLegacyLog, fmt.Sprintf(format, args...), nil, observability.CorrelationContext{})
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.Record(observability.LevelInfo, "client", observability.EventLegacyLog, fmt.Sprintf(format, args...), nil, observability.CorrelationContext{})
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Record(observability.LevelWarn, "client", observability.EventLegacyLog, fmt.Sprintf(format, args...), nil, observability.CorrelationContext{})
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Record(observability.LevelError, "client", observability.EventLegacyLog, fmt.Sprintf(format, args...), nil, observability.CorrelationContext{})
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Record(observability.LevelDebug, "client", observability.EventLegacyLog, fmt.Sprintf(format, args...), nil, observability.CorrelationContext{})
}

func (l *Logger) PluginLog(pluginName, level, format string, args ...interface{}) {
	l.Record(level, "plugin:"+pluginName, observability.EventLegacyLog, fmt.Sprintf(format, args...), nil, observability.CorrelationContext{})
}

func (l *Logger) Record(level, component, eventCode, message string, fields map[string]string, corr observability.CorrelationContext) {
	if l == nil || l.store == nil {
		return
	}
	record := observability.DiagnosticRecord{
		Level:     level,
		Component: component,
		EventCode: eventCode,
		Message:   message,
		Fields:    fields,
		Corr:      corr,
	}.Normalize(time.Now())
	if err := l.store.Record(record); err != nil {
		return
	}
	legacy := legacyLogEntry(record)

	l.observerMu.RLock()
	observers := make([]func(observedRecord), 0, len(l.observers))
	for _, fn := range l.observers {
		observers = append(observers, fn)
	}
	l.observerMu.RUnlock()
	for _, fn := range observers {
		fn(observedRecord{Diagnostic: record, Legacy: legacy})
	}
}

func (l *Logger) AddObserver(fn func(protocol.LogEntry)) func() {
	if fn == nil {
		return func() {}
	}
	return l.AddDiagnosticObserver(func(record observability.DiagnosticRecord) {
		fn(legacyLogEntry(record))
	})
}

func (l *Logger) AddDiagnosticObserver(fn func(observability.DiagnosticRecord)) func() {
	if fn == nil {
		return func() {}
	}
	l.observerMu.Lock()
	id := l.nextObsID
	l.nextObsID++
	l.observers[id] = func(record observedRecord) {
		fn(record.Diagnostic)
	}
	l.observerMu.Unlock()
	return func() {
		l.observerMu.Lock()
		delete(l.observers, id)
		l.observerMu.Unlock()
	}
}

func (l *Logger) GetRecentLogs(lines int, level string) []protocol.LogEntry {
	records := l.store.Tail(lines)
	out := make([]protocol.LogEntry, 0, len(records))
	for _, record := range records {
		if level != "" && record.Level != level {
			continue
		}
		out = append(out, legacyLogEntry(record))
	}
	return out
}

func (l *Logger) Query(query observability.LogQuery) (observability.LogPage, error) {
	return l.store.Query(query)
}

func (l *Logger) Follow(query observability.LogQuery) (<-chan observability.DiagnosticRecord, func(), error) {
	return l.store.Follow(query)
}

func (l *Logger) Close() {}

func legacyLogEntry(record observability.DiagnosticRecord) protocol.LogEntry {
	return protocol.LogEntry{
		Timestamp: record.Timestamp,
		Level:     record.Level,
		Message:   record.Message,
		Source:    record.Component,
		EventCode: record.EventCode,
		NodeRole:  record.NodeRole,
		NodeID:    record.NodeID,
		Fields:    record.Fields,
		Corr:      record.Corr,
	}
}
