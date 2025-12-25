package utils

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// PortManager 端口管理器
type PortManager struct {
	mu       sync.RWMutex
	occupied map[int]string // port -> clientID
}

// NewPortManager 创建端口管理器
func NewPortManager() *PortManager {
	return &PortManager{
		occupied: make(map[int]string),
	}
}

// IsPortAvailable 检测端口是否可用（系统级）
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// Reserve 预留端口给指定客户端
func (pm *PortManager) Reserve(port int, clientID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if owner, exists := pm.occupied[port]; exists {
		return fmt.Errorf("port %d already occupied by client %s", port, owner)
	}

	if !IsPortAvailable(port) {
		return fmt.Errorf("port %d is occupied by system", port)
	}

	pm.occupied[port] = clientID
	return nil
}

// Release 释放端口
func (pm *PortManager) Release(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.occupied, port)
}

// ReleaseByClient 释放指定客户端的所有端口
func (pm *PortManager) ReleaseByClient(clientID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for port, owner := range pm.occupied {
		if owner == clientID {
			delete(pm.occupied, port)
		}
	}
}

// CheckLocalService 检测本地服务是否可连接
func CheckLocalService(ip string, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("cannot connect to local service %s: %v", addr, err)
	}
	conn.Close()
	return nil
}
