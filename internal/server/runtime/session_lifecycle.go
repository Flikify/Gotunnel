package runtime

import (
	"log"
	"net"
	"time"

	domain "github.com/gotunnel/internal/core/domain"
	"github.com/gotunnel/pkg/observability"
	"github.com/hashicorp/yamux"
)

type sessionLifecycle struct {
	validateProxyRuleLimit func([]domain.ProxyRule) error
	registerClient         func(*ClientSession)
	unregisterClient       func(*ClientSession)
	startProxyListeners    func(*ClientSession)
	emitOperationalEvent   func(string, string, string, string, map[string]string, observability.CorrelationContext)
	channel                *controlChannel
	acceptClientStreams    func(*ClientSession)
	runtimeConfig          func() (heartbeatSec, heartbeatTimeoutSec, maxClientProxies int, responseTimeout time.Duration)
}

func newSessionLifecycle(
	validateProxyRuleLimit func([]domain.ProxyRule) error,
	registerClient func(*ClientSession),
	unregisterClient func(*ClientSession),
	startProxyListeners func(*ClientSession),
	emitOperationalEvent func(string, string, string, string, map[string]string, observability.CorrelationContext),
	channel *controlChannel,
	acceptClientStreams func(*ClientSession),
	runtimeConfig func() (heartbeatSec, heartbeatTimeoutSec, maxClientProxies int, responseTimeout time.Duration),
) *sessionLifecycle {
	return &sessionLifecycle{
		validateProxyRuleLimit: validateProxyRuleLimit,
		registerClient:         registerClient,
		unregisterClient:       unregisterClient,
		startProxyListeners:    startProxyListeners,
		emitOperationalEvent:   emitOperationalEvent,
		channel:                channel,
		acceptClientStreams:    acceptClientStreams,
		runtimeConfig:          runtimeConfig,
	}
}

func (l *sessionLifecycle) run(conn net.Conn, client *admittedClient) {
	session, err := yamux.Server(conn, nil)
	if err != nil {
		log.Printf("[Server] Yamux error: %v", err)
		return
	}
	if err := l.validateProxyRuleLimit(client.Rules); err != nil {
		log.Printf("[Server] Client %s rule limit validation failed: %v", client.ID, err)
		_ = session.Close()
		return
	}

	cs := newClientSession(session, client.ID, client.Name, client.RemoteAddr, client.OS, client.Arch, client.Version, client.Rules)
	l.registerClient(cs)
	defer l.unregisterClient(cs)
	l.emitOperationalEvent(
		observability.SeverityInfo,
		observability.CategoryLifecycle,
		observability.EventServerClientConnected,
		"Client connected",
		map[string]string{"client_id": client.ID, "remote_addr": client.RemoteAddr},
		observability.CorrelationContext{ClientID: client.ID},
	)

	l.startProxyListeners(cs)
	if err := l.channel.sendProxyConfig(session, cs.rulesSnapshot()); err != nil {
		log.Printf("[Server] Send config error: %v", err)
		return
	}

	go l.heartbeatLoop(cs)
	go l.acceptClientStreams(cs)

	<-session.CloseChan()
	l.emitOperationalEvent(
		observability.SeverityWarning,
		observability.CategoryLifecycle,
		observability.EventServerClientDisconnected,
		"Client disconnected",
		map[string]string{"client_id": client.ID},
		observability.CorrelationContext{ClientID: client.ID},
	)
	log.Printf("[Server] Client %s disconnected", client.ID)
}

func (l *sessionLifecycle) heartbeatLoop(cs *ClientSession) {
	heartbeatSec, heartbeatTimeoutSec, _, _ := l.runtimeConfig()
	ticker := time.NewTicker(time.Duration(heartbeatSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			heartbeatSec, heartbeatTimeoutSec, _, _ = l.runtimeConfig()
			ticker.Reset(time.Duration(heartbeatSec) * time.Second)
			timeout := time.Duration(heartbeatTimeoutSec) * time.Second

			if cs.heartbeatExpired(time.Now(), timeout) {
				l.emitOperationalEvent(
					observability.SeverityError,
					observability.CategoryHealth,
					observability.EventServerHeartbeatTimeout,
					"Client heartbeat timeout",
					map[string]string{"client_id": cs.ID},
					observability.CorrelationContext{ClientID: cs.ID},
				)
				log.Printf("[Server] Client %s heartbeat timeout", cs.ID)
				_ = cs.Session.Close()
				return
			}

			if l.channel.sendHeartbeat(cs) {
				cs.updateLastPing(time.Now())
			}
		case <-cs.Session.CloseChan():
			return
		}
	}
}
