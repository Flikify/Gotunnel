package main

import (
	"path/filepath"
	"runtime"
	"testing"

	clientconfig "github.com/gotunnel/internal/client/config"
)

func TestDefaultServiceName(t *testing.T) {
	got := defaultServiceName()
	switch runtime.GOOS {
	case "windows":
		if got != "GoTunnelClient" {
			t.Fatalf("unexpected windows service name: %q", got)
		}
	case "darwin":
		if got != "com.gotunnel.client" {
			t.Fatalf("unexpected darwin service name: %q", got)
		}
	default:
		if got != "gotunnel-client" {
			t.Fatalf("unexpected unix service name: %q", got)
		}
	}
}

func TestNormalizeServiceCommandOptionsRequiresConfigForInstall(t *testing.T) {
	_, err := normalizeServiceCommandOptions(serviceCommandOptions{Action: "install"}, nil)
	if err == nil {
		t.Fatal("expected install without config path to fail")
	}
}

func TestNormalizeServiceCommandOptionsDerivesLogPathFromDataDir(t *testing.T) {
	cfg := &clientconfig.ClientConfig{DataDir: filepath.Join("testdata", "client")}
	got, err := normalizeServiceCommandOptions(serviceCommandOptions{Action: "status"}, cfg)
	if err != nil {
		t.Fatalf("normalizeServiceCommandOptions returned error: %v", err)
	}

	want := filepath.Join(mustAbs(t, cfg.DataDir), "service.log")
	if got.LogPath != want {
		t.Fatalf("unexpected log path: got %q want %q", got.LogPath, want)
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs returned error: %v", err)
	}
	return abs
}
