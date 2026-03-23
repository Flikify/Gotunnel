package sqlite

import (
	"path/filepath"
	"testing"

	corerule "github.com/gotunnel/internal/core/rule"
)

func TestSQLiteStorePersistsClientMetadata(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "gotunnel.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	client := &Client{
		ID:             "client-1",
		Nickname:       "demo",
		Rules:          []corerule.ProxyRule{{Name: "web", RemotePort: 8080}},
		LastRemoteAddr: "10.0.0.8",
		LastOS:         "linux",
		LastArch:       "amd64",
		LastVersion:    "1.2.3",
		LastOfflineAt:  1710000000,
	}

	if err := store.CreateClient(client); err != nil {
		t.Fatalf("CreateClient returned error: %v", err)
	}

	got, err := store.GetClient(client.ID)
	if err != nil {
		t.Fatalf("GetClient returned error: %v", err)
	}

	if got.LastRemoteAddr != client.LastRemoteAddr || got.LastOS != client.LastOS || got.LastArch != client.LastArch || got.LastVersion != client.LastVersion {
		t.Fatalf("unexpected persisted metadata: %+v", got)
	}
	if got.LastOfflineAt != client.LastOfflineAt {
		t.Fatalf("expected LastOfflineAt %d, got %d", client.LastOfflineAt, got.LastOfflineAt)
	}
}
