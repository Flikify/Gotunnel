package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// getMachineID 获取机器唯一标识
// 优先级：系统机器ID > MAC地址哈希
func getMachineID() string {
	// 尝试获取系统机器 ID
	if id := getSystemMachineID(); id != "" {
		return hashID(id)
	}

	// 备选：使用主网卡 MAC 地址
	if id := getMACAddress(); id != "" {
		return hashID(id)
	}

	// 都失败则返回空，让服务端生成
	return ""
}

// getSystemMachineID 获取系统机器 ID
func getSystemMachineID() string {
	switch runtime.GOOS {
	case "linux":
		return getLinuxMachineID()
	case "darwin":
		return getDarwinMachineID()
	case "windows":
		return getWindowsMachineID()
	default:
		return ""
	}
}

// getLinuxMachineID 获取 Linux 机器 ID
func getLinuxMachineID() string {
	// 优先读取 /etc/machine-id
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}
	// 备选 /var/lib/dbus/machine-id
	if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getDarwinMachineID 获取 macOS 机器 ID (IOPlatformUUID)
func getDarwinMachineID() string {
	cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// 解析 IOPlatformUUID
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				uuid := strings.TrimSpace(parts[1])
				uuid = strings.Trim(uuid, "\"")
				return uuid
			}
		}
	}
	return ""
}

// getWindowsMachineID 获取 Windows 机器 ID
func getWindowsMachineID() string {
	cmd := exec.Command("reg", "query",
		`HKLM\SOFTWARE\Microsoft\Cryptography`,
		"/v", "MachineGuid")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// 解析注册表输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "MachineGuid") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[len(fields)-1]
			}
		}
	}
	return ""
}

// getMACAddress 获取主网卡 MAC 地址
func getMACAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过回环和无效接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}

		// 返回第一个有效的 MAC 地址
		return iface.HardwareAddr.String()
	}
	return ""
}

// hashID 对 ID 进行哈希处理，生成固定长度的客户端 ID
func hashID(id string) string {
	hash := sha256.Sum256([]byte(id))
	// 取前 16 个字符作为客户端 ID
	return hex.EncodeToString(hash[:])[:16]
}
