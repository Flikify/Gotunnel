package observability

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiagnosticStoreQueryAndFollow(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiagnosticStore(StoreOptions{
		RootDir:       dir,
		RetentionDays: 7,
		NodeID:        "node-1",
		NodeRole:      NodeRoleClient,
	})
	if err != nil {
		t.Fatalf("NewDiagnosticStore returned error: %v", err)
	}

	now := time.Now()
	records := []DiagnosticRecord{
		{Timestamp: now.Add(-2 * time.Minute).UnixMilli(), Level: LevelInfo, Component: "conn", EventCode: EventClientDialStarted, Message: "dial"},
		{Timestamp: now.Add(-1 * time.Minute).UnixMilli(), Level: LevelWarn, Component: "conn", EventCode: EventClientReconnectBackoff, Message: "retry"},
		{Timestamp: now.UnixMilli(), Level: LevelError, Component: "auth", EventCode: EventClientAuthRejected, Message: "auth failed"},
	}
	for _, record := range records {
		if err := store.Record(record); err != nil {
			t.Fatalf("Record returned error: %v", err)
		}
	}

	page, err := store.Query(LogQuery{Level: LevelWarn, Limit: 10})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].EventCode != EventClientReconnectBackoff {
		t.Fatalf("unexpected warn records: %+v", page.Records)
	}

	ch, cancel, err := store.Follow(LogQuery{Component: "auth"})
	if err != nil {
		t.Fatalf("Follow returned error: %v", err)
	}
	defer cancel()

	if err := store.Record(DiagnosticRecord{
		Timestamp: now.Add(time.Minute).UnixMilli(),
		Level:     LevelError,
		Component: "auth",
		EventCode: EventClientAuthRejected,
		Message:   "another auth failed",
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	select {
	case record := <-ch:
		if record.Component != "auth" {
			t.Fatalf("unexpected followed record: %+v", record)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for followed record")
	}
}

func TestDiagnosticStoreSkipsCorruptManifestAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	expiredDir := filepath.Join(dir, time.Now().AddDate(0, 0, -10).Format("2006-01-02"))
	if err := os.MkdirAll(expiredDir, 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(expiredDir, "00.ndjson"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	store, err := NewDiagnosticStore(StoreOptions{
		RootDir:       dir,
		RetentionDays: 3,
		NodeID:        "server",
		NodeRole:      NodeRoleServer,
	})
	if err != nil {
		t.Fatalf("NewDiagnosticStore returned error: %v", err)
	}
	_ = store

	if _, err := os.Stat(expiredDir); !os.IsNotExist(err) {
		t.Fatalf("expected expired directory to be removed, stat err=%v", err)
	}

	now := time.Now()
	if err := store.Record(DiagnosticRecord{
		Timestamp: now.UnixMilli(),
		Level:     LevelInfo,
		Component: "server",
		EventCode: EventServerClientConnected,
		Message:   "connected",
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	manifestPath := filepath.Join(dir, now.Format("2006-01-02"), now.Format("15")+".manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	page, err := store.Query(LogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(page.Records) != 1 {
		t.Fatalf("expected one record, got %d", len(page.Records))
	}
}
