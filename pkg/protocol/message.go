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
}

// AuthResponse 认证响应
type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ProxyRule 代理规则
type ProxyRule struct {
	Name       string `json:"name" yaml:"name"`
	Type       string `json:"type" yaml:"type"`               // 内置: tcp, udp, http, https; 插件: socks5 等
	LocalIP    string `json:"local_ip" yaml:"local_ip"`       // tcp/udp 模式使用
	LocalPort  int    `json:"local_port" yaml:"local_port"`   // tcp/udp 模式使用
	RemotePort int    `json:"remote_port" yaml:"remote_port"` // 服务端监听端口
	// Plugin 支持字段
	PluginName    string            `json:"plugin_name,omitempty" yaml:"plugin_name"`
	PluginVersion string            `json:"plugin_version,omitempty" yaml:"plugin_version"`
	PluginConfig  map[string]string `json:"plugin_config,omitempty" yaml:"plugin_config"`
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
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Checksum    string   `json:"checksum"`
	Size        int64    `json:"size"`
	Description string   `json:"description,omitempty"`
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

// UDPPacket UDP 数据包
type UDPPacket struct {
	RemotePort int    `json:"remote_port"` // 服务端监听端口
	ClientAddr string `json:"client_addr"` // 客户端地址 (用于回复)
	Data       []byte `json:"data"`        // UDP 数据
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
