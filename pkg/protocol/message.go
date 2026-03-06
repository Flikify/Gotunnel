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

	// Plugin 相关消息
	MsgTypePluginList     uint8 = 20 // 请求/响应可用 plugins
	MsgTypePluginDownload uint8 = 21 // 请求下载 plugin
	MsgTypePluginData     uint8 = 22 // Plugin 二进制数据（分块）
	MsgTypePluginReady    uint8 = 23 // Plugin 加载确认

	// UDP 相关消息
	MsgTypeUDPData uint8 = 30 // UDP 数据包

	// 插件安装消息
	MsgTypeInstallPlugins uint8 = 24 // 服务端推送安装插件列表
	MsgTypePluginConfig   uint8 = 25 // 插件配置同步

	// 客户端插件消息
	MsgTypeClientPluginStart     uint8 = 40 // 启动客户端插件
	MsgTypeClientPluginStop      uint8 = 41 // 停止客户端插件
	MsgTypeClientPluginStatus    uint8 = 42 // 客户端插件状态
	MsgTypeClientPluginConn      uint8 = 43 // 客户端插件连接请求
	MsgTypePluginStatusQuery     uint8 = 44 // 查询所有插件状态
	MsgTypePluginStatusQueryResp uint8 = 45 // 插件状态查询响应

	// JS 插件动态安装
	MsgTypeJSPluginInstall uint8 = 50 // 安装 JS 插件
	MsgTypeJSPluginResult  uint8 = 51 // 安装结果

	// 客户端控制消息
	MsgTypeClientRestart      uint8 = 60 // 重启客户端
	MsgTypePluginConfigUpdate uint8 = 61 // 更新插件配置

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

	// 插件 API 路由消息
	MsgTypePluginAPIRequest  uint8 = 90 // 插件 API 请求
	MsgTypePluginAPIResponse uint8 = 91 // 插件 API 响应

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
	Type       string `json:"type" yaml:"type"`                 // 内置: tcp, udp, http, https, websocket; 插件: socks5 等
	LocalIP    string `json:"local_ip" yaml:"local_ip"`         // tcp/udp 模式使用
	LocalPort  int    `json:"local_port" yaml:"local_port"`     // tcp/udp 模式使用
	RemotePort int    `json:"remote_port" yaml:"remote_port"`   // 服务端监听端口
	Enabled    *bool  `json:"enabled,omitempty" yaml:"enabled"` // 是否启用，默认为 true
	// Plugin 支持字段
	PluginID      string            `json:"plugin_id,omitempty" yaml:"plugin_id"` // 插件实例ID
	PluginName    string            `json:"plugin_name,omitempty" yaml:"plugin_name"`
	PluginVersion string            `json:"plugin_version,omitempty" yaml:"plugin_version"`
	PluginConfig  map[string]string `json:"plugin_config,omitempty" yaml:"plugin_config"`
	// HTTP Basic Auth 字段 (用于独立端口模式)
	AuthEnabled  bool   `json:"auth_enabled,omitempty" yaml:"auth_enabled"`
	AuthUsername string `json:"auth_username,omitempty" yaml:"auth_username"`
	AuthPassword string `json:"auth_password,omitempty" yaml:"auth_password"`
	// 插件管理标记 - 由插件自动创建的规则，不允许手动编辑/删除
	PluginManaged bool `json:"plugin_managed,omitempty" yaml:"plugin_managed"`
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

// PluginMetadata Plugin 元数据（协议层）
type PluginMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Checksum    string `json:"checksum"`
	Size        int64  `json:"size"`
	Description string `json:"description,omitempty"`
}

// PluginListRequest 请求可用 plugins
type PluginListRequest struct {
	ClientVersion string `json:"client_version"`
}

// PluginListResponse 返回可用 plugins
type PluginListResponse struct {
	Plugins []PluginMetadata `json:"plugins"`
}

// PluginDownloadRequest 请求下载 plugin
type PluginDownloadRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PluginDataChunk Plugin 二进制数据块
type PluginDataChunk struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	ChunkIndex  int    `json:"chunk_index"`
	TotalChunks int    `json:"total_chunks"`
	Data        []byte `json:"data"`
	Checksum    string `json:"checksum,omitempty"`
}

// PluginReadyNotification Plugin 加载确认
type PluginReadyNotification struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// InstallPluginsRequest 安装插件请求
type InstallPluginsRequest struct {
	Plugins []string `json:"plugins"` // 要安装的插件名称列表
}

// PluginConfigSync 插件配置同步
type PluginConfigSync struct {
	PluginName string            `json:"plugin_name"` // 插件名称
	Config     map[string]string `json:"config"`      // 配置内容
}

// UDPPacket UDP 数据包
type UDPPacket struct {
	RemotePort int    `json:"remote_port"` // 服务端监听端口
	ClientAddr string `json:"client_addr"` // 客户端地址 (用于回复)
	Data       []byte `json:"data"`        // UDP 数据
}

// ClientPluginStartRequest 启动客户端插件请求
type ClientPluginStartRequest struct {
	PluginName string            `json:"plugin_name"` // 插件名称
	RuleName   string            `json:"rule_name"`   // 规则名称
	RemotePort int               `json:"remote_port"` // 服务端监听端口
	Config     map[string]string `json:"config"`      // 插件配置
}

// ClientPluginStopRequest 停止客户端插件请求
type ClientPluginStopRequest struct {
	PluginID   string `json:"plugin_id,omitempty"` // 插件ID（优先使用）
	PluginName string `json:"plugin_name"`         // 插件名称
	RuleName   string `json:"rule_name"`           // 规则名称
}

// ClientPluginStatusResponse 客户端插件状态响应
type ClientPluginStatusResponse struct {
	PluginName string `json:"plugin_name"` // 插件名称
	RuleName   string `json:"rule_name"`   // 规则名称
	Running    bool   `json:"running"`     // 是否运行中
	LocalAddr  string `json:"local_addr"`  // 本地监听地址
	Error      string `json:"error"`       // 错误信息
}

// ClientPluginConnRequest 客户端插件连接请求
type ClientPluginConnRequest struct {
	PluginID   string `json:"plugin_id,omitempty"` // 插件ID（优先使用）
	PluginName string `json:"plugin_name"`         // 插件名称
	RuleName   string `json:"rule_name"`           // 规则名称
}

// PluginStatusEntry 单个插件状态
type PluginStatusEntry struct {
	PluginName string `json:"plugin_name"` // 插件名称
	Running    bool   `json:"running"`     // 是否运行中
}

// PluginStatusQueryResponse 插件状态查询响应
type PluginStatusQueryResponse struct {
	Plugins []PluginStatusEntry `json:"plugins"` // 所有插件状态
}

// JSPluginInstallRequest JS 插件安装请求
type JSPluginInstallRequest struct {
	PluginID   string            `json:"plugin_id"`   // 插件实例唯一 ID
	PluginName string            `json:"plugin_name"` // 插件名称
	Source     string            `json:"source"`      // JS 源码
	Signature  string            `json:"signature"`   // 官方签名 (Base64)
	RuleName   string            `json:"rule_name"`   // 规则名称
	RemotePort int               `json:"remote_port"` // 服务端监听端口
	Config     map[string]string `json:"config"`      // 插件配置
	AutoStart  bool              `json:"auto_start"`  // 是否自动启动
}

// JSPluginInstallResult JS 插件安装结果
type JSPluginInstallResult struct {
	PluginName string `json:"plugin_name"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
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

// PluginConfigUpdateRequest 插件配置更新请求
type PluginConfigUpdateRequest struct {
	PluginID   string            `json:"plugin_id,omitempty"` // 插件ID（优先使用）
	PluginName string            `json:"plugin_name"`         // 插件名称
	RuleName   string            `json:"rule_name"`           // 规则名称
	Config     map[string]string `json:"config"`              // 新配置
	Restart    bool              `json:"restart"`             // 是否重启插件
}

// PluginConfigUpdateResponse 插件配置更新响应
type PluginConfigUpdateResponse struct {
	PluginName string `json:"plugin_name"`
	RuleName   string `json:"rule_name"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
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
	Source    string `json:"src"`   // 来源: client, plugin:<name>
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

// PluginAPIRequest 插件 API 请求
type PluginAPIRequest struct {
	PluginID   string            `json:"plugin_id"`   // 插件实例唯一 ID
	PluginName string            `json:"plugin_name"` // 插件名称 (向后兼容)
	Method     string            `json:"method"`      // HTTP 方法: GET, POST, PUT, DELETE
	Path       string            `json:"path"`        // 路由路径
	Query      string            `json:"query"`       // 查询参数
	Headers    map[string]string `json:"headers"`     // 请求头
	Body       string            `json:"body"`        // 请求体
}

// PluginAPIResponse 插件 API 响应
type PluginAPIResponse struct {
	Status  int               `json:"status"`  // HTTP 状态码
	Headers map[string]string `json:"headers"` // 响应头
	Body    string            `json:"body"`    // 响应体
	Error   string            `json:"error"`   // 错误信息
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
