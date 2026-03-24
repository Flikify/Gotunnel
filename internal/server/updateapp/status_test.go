package updateapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gotunnel/pkg/version"
)

func TestGetServerUpdateStatusReturnsIdleByDefault(t *testing.T) {
	reset := setServerUpdateStatusPathForTest(t)
	defer reset()

	status, err := GetServerUpdateStatus()
	if err != nil {
		t.Fatalf("GetServerUpdateStatus returned error: %v", err)
	}

	if status.State != ServerUpdateStateIdle {
		t.Fatalf("expected idle state, got %q", status.State)
	}
	if status.CurrentVersion != version.Version {
		t.Fatalf("expected current version %q, got %q", version.Version, status.CurrentVersion)
	}
}

func TestGetServerUpdateStatusPromotesRestartingToSucceeded(t *testing.T) {
	reset := setServerUpdateStatusPathForTest(t)
	defer reset()

	previousVersion := version.Version
	version.Version = "v1.2.3"
	defer func() {
		version.Version = previousVersion
	}()

	raw := ServerUpdateStatus{
		State:         ServerUpdateStateRestarting,
		Message:       "正在重启",
		TargetVersion: "v1.2.3",
		StartedAt:     10,
		UpdatedAt:     20,
	}
	if err := writeRawStatusForTest(raw); err != nil {
		t.Fatalf("writeRawStatusForTest returned error: %v", err)
	}

	status, err := GetServerUpdateStatus()
	if err != nil {
		t.Fatalf("GetServerUpdateStatus returned error: %v", err)
	}

	if status.State != ServerUpdateStateSucceeded {
		t.Fatalf("expected succeeded state, got %q", status.State)
	}
	if status.CurrentVersion != "v1.2.3" {
		t.Fatalf("expected current version to be updated, got %q", status.CurrentVersion)
	}
	if status.FinishedAt == 0 {
		t.Fatal("expected finished_at to be populated")
	}
}

func setServerUpdateStatusPathForTest(t *testing.T) func() {
	t.Helper()

	previousPath := serverUpdateStatusPath
	serverUpdateStatusPath = filepath.Join(t.TempDir(), "server-update-status.json")

	return func() {
		serverUpdateStatusPath = previousPath
	}
}

func writeRawStatusForTest(status ServerUpdateStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return os.WriteFile(serverUpdateStatusPath, data, 0o644)
}
