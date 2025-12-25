package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
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
	LocalIP    string `json:"local_ip" yaml:"local_ip"`
	LocalPort  int    `json:"local_port" yaml:"local_port"`
	RemotePort int    `json:"remote_port" yaml:"remote_port"`
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

// WriteMessage 写入消息到 writer
func WriteMessage(w io.Writer, msg *Message) error {
	header := make([]byte, 5)
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
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:])

	if length > 1024*1024 {
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
