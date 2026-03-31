package tunnel

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

const clientIDFileName = "client.id"

func resolveDataDir(explicit string) string {
	if explicit != "" {
		return explicit
	}

	if envDir := strings.TrimSpace(os.Getenv("GOTUNNEL_DATA_DIR")); envDir != "" {
		return envDir
	}

	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "gotunnel")
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".gotunnel")
	}

	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		return filepath.Join(cwd, ".gotunnel")
	}

	return ".gotunnel"
}

func resolveClientName(explicit string) string {
	if explicit != "" {
		return explicit
	}

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}

	if runtime.GOOS == "android" {
		return "android-device"
	}

	return "gotunnel-client"
}

func resolveClientID(dataDir, explicit string) string {
	if explicit != "" {
		return explicit
	}

	if id := loadClientID(dataDir); id != "" {
		return id
	}

	if id := getMachineID(); id != "" {
		_ = persistClientID(dataDir, id)
		return id
	}

	id := strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	_ = persistClientID(dataDir, id)
	return id
}

func loadClientID(dataDir string) string {
	data, err := os.ReadFile(filepath.Join(dataDir, clientIDFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func persistClientID(dataDir, id string) error {
	if id == "" {
		return nil
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, clientIDFileName), []byte(id+"\n"), 0600)
}
