package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gotunnel/pkg/observability"
)

func TestSQLiteStoreOperationalEvents(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	now := time.Now().UnixMilli()
	err = store.AppendOperationalEvents([]observability.OperationalEvent{
		{
			Timestamp: now,
			Severity:  observability.SeverityInfo,
			NodeID:    "client-1",
			NodeRole:  observability.NodeRoleClient,
			Category:  observability.CategoryLifecycle,
			EventCode: observability.EventClientSessionEstablished,
			Summary:   "connected",
		},
		{
			Timestamp: now + 1,
			Severity:  observability.SeverityCritical,
			NodeID:    "client-1",
			NodeRole:  observability.NodeRoleClient,
			Category:  observability.CategoryIncident,
			EventCode: observability.EventIncidentReconnectStorm,
			Summary:   "storm",
		},
	})
	if err != nil {
		t.Fatalf("AppendOperationalEvents returned error: %v", err)
	}

	events, err := store.ListOperationalEvents(observability.EventFilter{NodeID: "client-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListOperationalEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	health, err := store.ListNodeHealth(10)
	if err != nil {
		t.Fatalf("ListNodeHealth returned error: %v", err)
	}
	if len(health) != 1 {
		t.Fatalf("expected 1 node health row, got %d", len(health))
	}
	if health[0].IncidentCounts[observability.EventIncidentReconnectStorm] != 1 {
		t.Fatalf("unexpected incident counts: %+v", health[0].IncidentCounts)
	}
}
