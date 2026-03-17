package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

// 协议常量
const (
	MaxMessageSize = 1024 * 1024 // 最大消息大小 1MB
	HeaderSize     = 5           // 消息头大小
)

// 消息类型定义
const (
	MsgTypeAuth         uint8 = 1  // 认证请求
	MsgTypeAuthResp     uint8 = 2  // 认证响应
	MsgTypeProxyConfig  uint8 = 3  // 代理配置下发
	MsgTypeHeartbeat    uint8 = 4  // 心跳
	MsgTypeHeartbeatAck uint8 = 5  // 心跳响应
	MsgTypeNewProxy     uint8 = 6  // 新建代理连接请求
	MsgTypeProxyReady   uint8 = 7  // 代理就绪
	MsgTypeError        uint8 = 8  // 错误消息
	MsgTypeProxyConnect uint8 = 9  // 代理连接请求 (SOCKS5/HTTP)
	MsgTypeProxyResult  uint8 = 10 // 代理连接结果

	// UDP 相关消息
	MsgTypeUDPData uint8 = 30 // UDP 数据包

	// 客户端控制消息
	MsgTypeClientRestart uint8 = 60 // 重启客户端

	// 更新相关消息
	MsgTypeUpdateCheck    uint8 = 70 // 检查更新请求
	MsgTypeUpdateInfo     uint8 = 71 // 更新信息响应
	MsgTypeUpdateDownload uint8 = 72 // 下载更新请求
	MsgTypeUpdateApply    uint8 = 73 // 应用更新请求
	MsgTypeUpdateProgress uint8 = 74 // 更新进度
	MsgTypeUpdateResult   uint8 = 75 // 更新结果

	// 日志相关消息
	MsgTypeLogRequest uint8 = 80 // 请求客户端日志
	MsgTypeLogData    uint8 = 81 // 日志数据
	MsgTypeLogStop    uint8 = 82 // 停止日志流

	// 系统状态消息
	MsgTypeSystemStatsRequest  uint8 = 100 // 请求系统状态
	MsgTypeSystemStatsResponse uint8 = 101 // 系统状态响应

	// 截图消息
	MsgTypeScreenshotRequest  uint8 = 102 // 请求截图
	MsgTypeScreenshotResponse uint8 = 103 // 截图响应

	// Shell 执行消息
	MsgTypeShellExecuteRequest  uint8 = 104 // 执行 Shell 命令
	MsgTypeShellExecuteResponse uint8 = 105 // Shell 执行结果
)

// Message 基础消息结构
type Message struct {
	Type    uint8  `json:"type"`
	Payload []byte `json:"payload"`
}

// AuthRequest 认证请求
type AuthRequest struct {
	ClientID string `json:"client_id"`
	Token    string `json:"token"`
	Name     string `json:"name,omitempty"`    // 客户端名称（主机名）
	OS       string `json:"os,omitempty"`      // 客户端操作系统
	Arch     string `json:"arch,omitempty"`    // 客户端架构
	Version  string `json:"version,omitempty"` // 客户端版本
}

// AuthResponse 认证响应
type AuthResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	ClientID string `json:"client_id,omitempty"` // 服务端分配的客户端 ID
}

// ProxyRule 代理规则
type ProxyRule struct {
	Name       string `json:"name" yaml:"name"`
	Type       string `json:"type" yaml:"type"`                 // tcp, udp, http, https, socks5
	LocalIP    string `json:"local_ip" yaml:"local_ip"`         // tcp/udp 模式使用
	LocalPort  int    `json:"local_port" yaml:"local_port"`     // tcp/udp 模式使用
	RemotePort int    `json:"remote_port" yaml:"remote_port"`   // 服务端监听端口
	Enabled    *bool  `json:"enabled,omitempty" yaml:"enabled"` // 是否启用，默认为 true
	// HTTP Basic Auth 字段
	AuthEnabled  bool   `json:"auth_enabled,omitempty" yaml:"auth_enabled"`
	AuthUsername string `json:"auth_username,omitempty" yaml:"auth_username"`
	AuthPassword string `json:"auth_password,omitempty" yaml:"auth_password"`
	// 端口状态: "listening", "failed: <error message>", ""
	PortStatus string `json:"port_status,omitempty" yaml:"-"`
}

// IsEnabled 检查规则是否启用，默认为 true
func (r *ProxyRule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// ProxyConfig 代理配置下发
type ProxyConfig struct {
	Rules []ProxyRule `json:"rules"`
}

// NewProxyRequest 新建代理连接请求
type NewProxyRequest struct {
	RemotePort int `json:"remote_port"`
}

// ErrorMessage 错误消息
type ErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ProxyConnectRequest 代理连接请求
type ProxyConnectRequest struct {
	Target string `json:"target"` // 目标地址 host:port
}

// ProxyConnectResult 代理连接结果
type ProxyConnectResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// UDPPacket UDP 数据包
type UDPPacket struct {
	RemotePort int    `json:"remote_port"` // 服务端监听端口
	ClientAddr string `json:"client_addr"` // 客户端地址 (用于回复)
	Data       []byte `json:"data"`        // UDP 数据
}

// ClientRestartRequest 客户端重启请求
type ClientRestartRequest struct {
	Reason string `json:"reason,omitempty"` // 重启原因
}

// ClientRestartResponse 客户端重启响应
type ClientRestartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// UpdateCheckRequest 更新检查请求
type UpdateCheckRequest struct {
	Component string `json:"component"` // "server" 或 "client"
}

// UpdateInfoResponse 更新信息响应
type UpdateInfoResponse struct {
	Available   bool   `json:"available"`
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	ReleaseNote string `json:"release_note"`
	DownloadURL string `json:"download_url"`
	AssetName   string `json:"asset_name"`
	AssetSize   int64  `json:"asset_size"`
}

// UpdateDownloadRequest 下载更新请求
type UpdateDownloadRequest struct {
	DownloadURL string `json:"download_url"`
}

// UpdateApplyRequest 应用更新请求
type UpdateApplyRequest struct {
	Restart bool `json:"restart"` // 是否自动重启
}

// UpdateProgressResponse 更新进度响应
type UpdateProgressResponse struct {
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	Status     string `json:"status"` // downloading, applying, completed, failed
}

// UpdateResultResponse 更新结果响应
type UpdateResultResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// LogRequest 日志请求
type LogRequest struct {
	SessionID string `json:"session_id"` // 会话 ID
	Lines     int    `json:"lines"`      // 请求的日志行数
	Follow    bool   `json:"follow"`     // 是否持续推送新日志
	Level     string `json:"level"`      // 日志级别过滤
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp int64  `json:"ts"`    // Unix 时间戳 (毫秒)
	Level     string `json:"level"` // 日志级别: debug, info, warn, error
	Message   string `json:"msg"`   // 日志消息
	Source    string `json:"src"`   // 来源: client
}

// LogData 日志数据
type LogData struct {
	SessionID string     `json:"session_id"` // 会话 ID
	Entries   []LogEntry `json:"entries"`    // 日志条目
	EOF       bool       `json:"eof"`        // 是否结束
}

// LogStopRequest 停止日志流请求
type LogStopRequest struct {
	SessionID string `json:"session_id"` // 会话 ID
}

// WriteMessage 写入消息到 writer
func WriteMessage(w io.Writer, msg *Message) error {
	header := make([]byte, HeaderSize)
	header[0] = msg.Type
	binary.BigEndian.PutUint32(header[1:], uint32(len(msg.Payload)))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if len(msg.Payload) > 0 {
		if _, err := w.Write(msg.Payload); err != nil {
			return err
		}
	}
	return nil
}

// ReadMessage 从 reader 读取消息
func ReadMessage(r io.Reader) (*Message, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:])

	if length > MaxMessageSize {
		return nil, errors.New("message too large")
	}

	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	return &Message{Type: msgType, Payload: payload}, nil
}

// NewMessage 创建新消息
func NewMessage(msgType uint8, data interface{}) (*Message, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Message{Type: msgType, Payload: payload}, nil
}

// ParsePayload 解析消息载荷
func (m *Message) ParsePayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

// SystemStatsRequest 系统状态请求
type SystemStatsRequest struct{}

// SystemStatsResponse 系统状态响应
type SystemStatsResponse struct {
	CPUUsage    float64 `json:"cpu_usage"`    // CPU 使用率 (0-100)
	MemoryTotal uint64  `json:"memory_total"` // 总内存 (字节)
	MemoryUsed  uint64  `json:"memory_used"`  // 已用内存 (字节)
	MemoryUsage float64 `json:"memory_usage"` // 内存使用率 (0-100)
	DiskTotal   uint64  `json:"disk_total"`   // 总磁盘 (字节)
	DiskUsed    uint64  `json:"disk_used"`    // 已用磁盘 (字节)
	DiskUsage   float64 `json:"disk_usage"`   // 磁盘使用率 (0-100)
}

// ScreenshotRequest 截图请求
type ScreenshotRequest struct {
	Quality int `json:"quality"` // JPEG 质量 1-100, 0 使用默认值
}

// ScreenshotResponse 截图响应
type ScreenshotResponse struct {
	Data      string `json:"data"`            // Base64 编码的 JPEG 图片
	Width     int    `json:"width"`           // 图片宽度
	Height    int    `json:"height"`          // 图片高度
	Timestamp int64  `json:"timestamp"`       // 截图时间戳
	Error     string `json:"error,omitempty"` // 错误信息
}

// ShellExecuteRequest Shell 执行请求
type ShellExecuteRequest struct {
	Command string `json:"command"` // 要执行的命令
	Timeout int    `json:"timeout"` // 超时秒数, 0 使用默认值 (30秒)
}

// ShellExecuteResponse Shell 执行响应
type ShellExecuteResponse struct {
	Output   string `json:"output"`          // stdout + stderr 组合输出
	ExitCode int    `json:"exit_code"`       // 进程退出码
	Error    string `json:"error,omitempty"` // 错误信息
}
