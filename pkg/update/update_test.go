package update

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBinaryCandidateNames(t *testing.T) {
	tests := []struct {
		name      string
		component string
		goos      string
		want      []string
	}{
		{
			name:      "linux server",
			component: "server",
			goos:      "linux",
			want:      []string{"server"},
		},
		{
			name:      "windows client",
			component: "client",
			goos:      "windows",
			want:      []string{"client", "client.exe"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := binaryCandidateNames(tt.component, tt.goos)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected candidate names: got %v want %v", got, tt.want)
			}
		})
	}
}

func TestFindExtractedBinaryMatchesCurrentReleaseNaming(t *testing.T) {
	extractDir := t.TempDir()

	binaryPath := filepath.Join(extractDir, "server")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := FindExtractedBinary(extractDir, "server")
	if err != nil {
		t.Fatalf("FindExtractedBinary returned error: %v", err)
	}
	if got != binaryPath {
		t.Fatalf("unexpected binary path: got %q want %q", got, binaryPath)
	}
}

func TestFindExtractedBinaryMatchesLegacyNaming(t *testing.T) {
	extractDir := t.TempDir()

	binaryPath := filepath.Join(extractDir, "gotunnel-server-v1.2.9-linux-amd64")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := FindExtractedBinary(extractDir, "server")
	if err != nil {
		t.Fatalf("FindExtractedBinary returned error: %v", err)
	}
	if got != binaryPath {
		t.Fatalf("unexpected binary path: got %q want %q", got, binaryPath)
	}
}

func TestFindExtractedBinaryMatchesCurrentWindowsClientNaming(t *testing.T) {
	extractDir := t.TempDir()

	binaryPath := filepath.Join(extractDir, "gotunnel-client-v1.2.9-windows-amd64.zip")
	if err := os.WriteFile(binaryPath, []byte("archive"), 0644); err != nil {
		t.Fatalf("write archive placeholder: %v", err)
	}

	extractedBinary := filepath.Join(extractDir, "nested", "client.exe")
	if err := os.MkdirAll(filepath.Dir(extractedBinary), 0755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	if err := os.WriteFile(extractedBinary, []byte("binary"), 0644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := findExtractedBinary(extractDir, "client", "windows")
	if err != nil {
		t.Fatalf("FindExtractedBinary returned error: %v", err)
	}
	if got != extractedBinary {
		t.Fatalf("unexpected binary path: got %q want %q", got, extractedBinary)
	}
}

func TestFindExtractedBinaryIgnoresArchives(t *testing.T) {
	extractDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(extractDir, "gotunnel-server-v1.2.9-linux-amd64.tar.gz"), []byte("archive"), 0644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if _, err := FindExtractedBinary(extractDir, "server"); err == nil {
		t.Fatal("expected error when only archive files are present")
	}
}
