package utils

import (
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemStats 系统状态信息
type SystemStats struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskUsage   float64 `json:"disk_usage"`
}

// GetSystemStats 获取系统状态信息
func GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	// CPU 使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		stats.CPUUsage = cpuPercent[0]
	}

	// 内存信息
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		stats.MemoryTotal = memInfo.Total
		stats.MemoryUsed = memInfo.Used
		stats.MemoryUsage = memInfo.UsedPercent
	}

	// 磁盘信息 - 获取根目录或当前工作目录所在磁盘
	diskPath := "/"
	if runtime.GOOS == "windows" {
		// Windows 使用当前工作目录所在盘符
		if wd, err := os.Getwd(); err == nil && len(wd) >= 2 {
			diskPath = wd[:2] + "\\"
		} else {
			diskPath = "C:\\"
		}
	}

	diskInfo, err := disk.Usage(diskPath)
	if err == nil {
		stats.DiskTotal = diskInfo.Total
		stats.DiskUsed = diskInfo.Used
		stats.DiskUsage = diskInfo.UsedPercent
	}

	return stats, nil
}
