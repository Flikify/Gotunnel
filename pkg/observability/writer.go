package observability

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

type stdLogWriter struct {
	recorder  *DiagnosticStore
	component string
	fields    map[string]string
	mu        sync.Mutex
	buf       bytes.Buffer
}

func NewStdLogWriter(recorder *DiagnosticStore, component string, fields map[string]string) io.Writer {
	if fields == nil {
		fields = map[string]string{}
	}
	return &stdLogWriter{
		recorder:  recorder,
		component: component,
		fields:    fields,
	}
}

func (w *stdLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(p)
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line)
			break
		}
		w.recordLine(line)
	}
	return total, nil
}

func (w *stdLogWriter) recordLine(line string) {
	message := strings.TrimSpace(line)
	if message == "" {
		return
	}
	fields := make(map[string]string, len(w.fields))
	for key, value := range w.fields {
		fields[key] = value
	}
	_ = w.recorder.Record(DiagnosticRecord{
		Level:     LevelInfo,
		Component: w.component,
		EventCode: EventLegacyLog,
		Message:   message,
		Fields:    fields,
	})
}
