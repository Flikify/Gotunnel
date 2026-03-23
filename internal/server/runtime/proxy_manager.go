package runtime

import (
	"net"
	"sync"
	"time"

	"github.com/gotunnel/pkg/utils"
)

type clientProxyBindings struct {
	listeners map[int]net.Listener
	udpConns  map[int]*net.UDPConn
}

type proxyManager struct {
	mu                    sync.RWMutex
	bindings              map[string]*clientProxyBindings
	portManager           *utils.PortManager
	recordTraffic         func(inbound, outbound int64)
	clientResponseTimeout func() time.Duration
	requestProxyOpen      func(stream net.Conn, remotePort int) error
}

func newProxyManager(recordTraffic func(inbound, outbound int64), clientResponseTimeout func() time.Duration, requestProxyOpen func(stream net.Conn, remotePort int) error) *proxyManager {
	return &proxyManager{
		bindings:              make(map[string]*clientProxyBindings),
		portManager:           utils.NewPortManager(),
		recordTraffic:         recordTraffic,
		clientResponseTimeout: clientResponseTimeout,
		requestProxyOpen:      requestProxyOpen,
	}
}

func (m *proxyManager) bindTCPListener(clientID string, port int, listener net.Listener) {
	m.mu.Lock()
	bindings := m.ensureBindingsLocked(clientID)
	bindings.listeners[port] = listener
	m.mu.Unlock()
}

func (m *proxyManager) bindUDPConn(clientID string, port int, conn *net.UDPConn) {
	m.mu.Lock()
	bindings := m.ensureBindingsLocked(clientID)
	bindings.udpConns[port] = conn
	m.mu.Unlock()
}

func (m *proxyManager) stop(clientID string) {
	m.mu.Lock()
	bindings := m.bindings[clientID]
	delete(m.bindings, clientID)
	m.mu.Unlock()

	if bindings == nil {
		return
	}

	for port, listener := range bindings.listeners {
		_ = listener.Close()
		m.portManager.Release(port)
	}
	for port, conn := range bindings.udpConns {
		_ = conn.Close()
		m.portManager.Release(port)
	}
}

func (m *proxyManager) isPortAvailable(port int, excludeClientID string) bool {
	m.mu.RLock()
	for clientID, bindings := range m.bindings {
		if _, ok := bindings.listeners[port]; ok {
			m.mu.RUnlock()
			return clientID == excludeClientID
		}
		if _, ok := bindings.udpConns[port]; ok {
			m.mu.RUnlock()
			return clientID == excludeClientID
		}
	}
	m.mu.RUnlock()

	return utils.IsPortAvailable(port)
}

func (m *proxyManager) ensureBindingsLocked(clientID string) *clientProxyBindings {
	bindings, ok := m.bindings[clientID]
	if ok {
		return bindings
	}

	bindings = &clientProxyBindings{
		listeners: make(map[int]net.Listener),
		udpConns:  make(map[int]*net.UDPConn),
	}
	m.bindings[clientID] = bindings
	return bindings
}
