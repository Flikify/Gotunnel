package dto

import "github.com/gotunnel/internal/server/db"

// TrafficTotals represents aggregated inbound/outbound traffic.
type TrafficTotals struct {
	Inbound  int64 `json:"inbound"`
	Outbound int64 `json:"outbound"`
}

// TrafficStatsResponse returns 24h and total traffic counters.
type TrafficStatsResponse struct {
	Traffic24h   TrafficTotals `json:"traffic_24h"`
	TrafficTotal TrafficTotals `json:"traffic_total"`
}

// HourlyTrafficResponse returns hourly traffic samples.
type HourlyTrafficResponse struct {
	Records []db.TrafficRecord `json:"records"`
}

// LogEntry describes a single streamed client log line.
type LogEntry struct {
	Timestamp int64  `json:"ts"`
	Level     string `json:"level"`
	Message   string `json:"msg"`
	Source    string `json:"src"`
}

// SystemStatsResponse describes client system metrics.
type SystemStatsResponse struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskUsage   float64 `json:"disk_usage"`
}

// ScreenshotResponse describes a client screenshot payload.
type ScreenshotResponse struct {
	Data      string `json:"data"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}

// ShellExecuteResponse describes the result of a remote shell execution.
type ShellExecuteResponse struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// ExecuteShellRequest describes a remote shell command execution request.
type ExecuteShellRequest struct {
	Command string `json:"command" binding:"required"`
	Timeout int    `json:"timeout"`
}
