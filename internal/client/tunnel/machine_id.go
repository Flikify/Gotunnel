package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// getMachineID builds a stable fingerprint from multiple host identifiers
// and hashes the combined result into the client ID we expose externally.
func getMachineID() string {
	parts := collectMachineIDParts()
	if len(parts) == 0 {
		return ""
	}
	return hashID(strings.Join(parts, "|"))
}

func collectMachineIDParts() []string {
	parts := make([]string, 0, 6)

	if id := getSystemMachineID(); id != "" {
		parts = append(parts, "system="+id)
	}

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		parts = append(parts, "host="+hostname)
	}

	if macs := getMACAddresses(); len(macs) > 0 {
		parts = append(parts, "macs="+strings.Join(macs, ","))
	}

	if names := getInterfaceNames(); len(names) > 0 {
		parts = append(parts, "ifaces="+strings.Join(names, ","))
	}

	if len(parts) == 0 {
		return nil
	}

	parts = append(parts, "os="+runtime.GOOS, "arch="+runtime.GOARCH)
	return parts
}

func getSystemMachineID() string {
	switch runtime.GOOS {
	case "linux":
		return getLinuxMachineID()
	case "darwin":
		return getDarwinMachineID()
	case "windows":
		return getWindowsMachineID()
	case "android":
		return ""
	default:
		return ""
	}
}

func getLinuxMachineID() string {
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func getDarwinMachineID() string {
	cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, "IOPlatformUUID") {
			continue
		}

		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}

		uuid := strings.TrimSpace(parts[1])
		return strings.Trim(uuid, "\"")
	}

	return ""
}

func getWindowsMachineID() string {
	cmd := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, "MachineGuid") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			return fields[len(fields)-1]
		}
	}

	return ""
}

func getMACAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	macs := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		macs = append(macs, iface.HardwareAddr.String())
	}

	sort.Strings(macs)
	return macs
}

func getInterfaceNames() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		names = append(names, iface.Name)
	}

	sort.Strings(names)
	return names
}

func hashID(id string) string {
	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:])[:16]
}
