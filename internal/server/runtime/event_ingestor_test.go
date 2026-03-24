package runtime

import (
	"path/filepath"
	"testing"
	"time"

	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/observability"
)

func TestEventIngestorCreatesIncidentAfterRepeatedFailures(t *testing.T) {
	store, err := db.NewSQLiteStore(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	ingestor := newEventIngestor(store)
	now := time.Now()
	for i := 0; i < 3; i++ {
		err = ingestor.Ingest([]observability.OperationalEvent{{
			Timestamp: now.Add(time.Duration(i) * time.Second).UnixMilli(),
			Severity:  observability.SeverityError,
			NodeID:    "client-1",
			NodeRole:  observability.NodeRoleClient,
			Category:  observability.CategorySecurity,
			EventCode: observability.EventClientAuthRejected,
			Summary:   "auth rejected",
		}})
		if err != nil {
			t.Fatalf("Ingest returned error: %v", err)
		}
	}

	events, err := store.ListOperationalEvents(observability.EventFilter{
		NodeID:    "client-1",
		Category:  observability.CategoryIncident,
		EventCode: observability.EventIncidentAuthFailures,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListOperationalEvents returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one incident event, got %d", len(events))
	}
}
