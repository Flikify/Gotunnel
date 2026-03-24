package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
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

func TestSafeArchivePathRejectsTraversal(t *testing.T) {
	destDir := t.TempDir()

	if _, err := safeArchivePath(destDir, "../escape"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if _, err := safeArchivePath(destDir, "/absolute/path"); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
}

func TestExtractTarGzRejectsTraversalEntries(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	gzWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzWriter)
	payload := []byte("owned")
	header := &tar.Header{
		Name: "../escape.txt",
		Mode: 0644,
		Size: int64(len(payload)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close returned error: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("gzWriter.Close returned error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("file.Close returned error: %v", err)
	}

	if err := ExtractTarGz(archivePath, t.TempDir()); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
}

func TestExtractZipRejectsTraversalEntries(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	zipWriter := zip.NewWriter(file)
	entry, err := zipWriter.Create("../escape.txt")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := entry.Write([]byte("owned")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("zipWriter.Close returned error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("file.Close returned error: %v", err)
	}

	if err := ExtractZip(archivePath, t.TempDir()); err == nil {
		t.Fatal("expected traversal zip to be rejected")
	}
}

func TestExtractTarGzCreatesNestedFilesWithinDestination(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	gzWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzWriter)
	payload := []byte("binary")
	header := &tar.Header{
		Name: "nested/server",
		Mode: 0644,
		Size: int64(len(payload)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close returned error: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("gzWriter.Close returned error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("file.Close returned error: %v", err)
	}

	destDir := t.TempDir()
	if err := ExtractTarGz(archivePath, destDir); err != nil {
		t.Fatalf("ExtractTarGz returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "nested", "server"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("unexpected payload: got %q want %q", got, payload)
	}
}
