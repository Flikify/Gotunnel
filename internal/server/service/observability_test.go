package service

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	serverruntime "github.com/gotunnel/internal/server/runtime"
	"github.com/gotunnel/pkg/observability"
)

type fakeDiagnosticsRuntime struct {
	local *observability.DiagnosticStore
}

func (f *fakeDiagnosticsRuntime) IsClientOnline(clientID string) bool { return false }
func (f *fakeDiagnosticsRuntime) OpenClientStream(clientID string) (net.Conn, error) {
	return nil, nil
}
func (f *fakeDiagnosticsRuntime) ClientResponseTimeout() time.Duration { return time.Second }
func (f *fakeDiagnosticsRuntime) LogSessions() *serverruntime.LogSessionManager {
	return serverruntime.NewLogSessionManager()
}
func (f *fakeDiagnosticsRuntime) LocalDiagnosticStore() *observability.DiagnosticStore {
	return f.local
}

func TestDiagnosticsServiceQueryServerNode(t *testing.T) {
	store, err := observability.NewDiagnosticStore(observability.StoreOptions{
		RootDir:       filepath.Join(t.TempDir(), "server-diag"),
		RetentionDays: 3,
		NodeID:        "server",
		NodeRole:      observability.NodeRoleServer,
	})
	if err != nil {
		t.Fatalf("NewDiagnosticStore returned error: %v", err)
	}
	if err := store.Record(observability.DiagnosticRecord{
		Timestamp: time.Now().UnixMilli(),
		Level:     observability.LevelInfo,
		Component: "server",
		EventCode: observability.EventServerClientConnected,
		Message:   "client connected",
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	svc := NewDiagnosticsService(&fakeDiagnosticsRuntime{local: store})
	page, err := svc.QueryNodeDiagnostics("server", observability.LogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("QueryNodeDiagnostics returned error: %v", err)
	}
	if len(page.Records) != 1 {
		t.Fatalf("expected one diagnostic record, got %d", len(page.Records))
	}
}
