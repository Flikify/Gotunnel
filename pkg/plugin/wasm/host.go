package wasm

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gotunnel/pkg/plugin"
)

// ErrInvalidHandle 无效的连接句柄
var ErrInvalidHandle = errors.New("invalid connection handle")

// HostContextImpl 实现 HostContext 接口
type HostContextImpl struct {
	dialer     plugin.Dialer
	clientConn net.Conn
	config     map[string]string

	// 连接管理
	conns     map[plugin.ConnHandle]net.Conn
	nextHandle plugin.ConnHandle
	mu        sync.Mutex
}

// NewHostContext 创建 host context
func NewHostContext(dialer plugin.Dialer, clientConn net.Conn, config map[string]string) *HostContextImpl {
	return &HostContextImpl{
		dialer:     dialer,
		clientConn: clientConn,
		config:     config,
		conns:      make(map[plugin.ConnHandle]net.Conn),
		nextHandle: 1,
	}
}

// Dial 通过隧道建立连接
func (h *HostContextImpl) Dial(network, address string) (plugin.ConnHandle, error) {
	conn, err := h.dialer.Dial(network, address)
	if err != nil {
		return 0, err
	}

	h.mu.Lock()
	handle := h.nextHandle
	h.nextHandle++
	h.conns[handle] = conn
	h.mu.Unlock()

	return handle, nil
}

// Read 从连接读取数据
func (h *HostContextImpl) Read(handle plugin.ConnHandle, buf []byte) (int, error) {
	h.mu.Lock()
	conn, ok := h.conns[handle]
	h.mu.Unlock()

	if !ok {
		return 0, ErrInvalidHandle
	}

	return conn.Read(buf)
}

// Write 向连接写入数据
func (h *HostContextImpl) Write(handle plugin.ConnHandle, buf []byte) (int, error) {
	h.mu.Lock()
	conn, ok := h.conns[handle]
	h.mu.Unlock()

	if !ok {
		return 0, ErrInvalidHandle
	}

	return conn.Write(buf)
}

// CloseConn 关闭连接
func (h *HostContextImpl) CloseConn(handle plugin.ConnHandle) error {
	h.mu.Lock()
	conn, ok := h.conns[handle]
	if ok {
		delete(h.conns, handle)
	}
	h.mu.Unlock()

	if !ok {
		return ErrInvalidHandle
	}

	return conn.Close()
}

// ClientRead 从客户端连接读取数据
func (h *HostContextImpl) ClientRead(buf []byte) (int, error) {
	return h.clientConn.Read(buf)
}

// ClientWrite 向客户端连接写入数据
func (h *HostContextImpl) ClientWrite(buf []byte) (int, error) {
	return h.clientConn.Write(buf)
}

// Log 记录日志
func (h *HostContextImpl) Log(level plugin.LogLevel, message string) {
	prefix := "[WASM]"
	switch level {
	case plugin.LogDebug:
		prefix = "[WASM DEBUG]"
	case plugin.LogInfo:
		prefix = "[WASM INFO]"
	case plugin.LogWarn:
		prefix = "[WASM WARN]"
	case plugin.LogError:
		prefix = "[WASM ERROR]"
	}
	log.Printf("%s %s", prefix, message)
}

// Now 返回当前 Unix 时间戳
func (h *HostContextImpl) Now() int64 {
	return time.Now().Unix()
}

// GetConfig 获取配置值
func (h *HostContextImpl) GetConfig(key string) string {
	if h.config == nil {
		return ""
	}
	return h.config[key]
}

// Close 关闭所有连接
func (h *HostContextImpl) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for handle, conn := range h.conns {
		conn.Close()
		delete(h.conns, handle)
	}
	return nil
}
