package tunnel

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/domain"
)

type runtimeControl struct {
	clientStore            db.ClientStore
	sessions               *clientSessionRegistry
	proxies                *proxyManager
	logSessions            *LogSessionManager
	channel                *controlChannel
	validateProxyRuleLimit func([]domain.ProxyRule) error
}

func newRuntimeControl(
	clientStore db.ClientStore,
	sessions *clientSessionRegistry,
	proxies *proxyManager,
	logSessions *LogSessionManager,
	channel *controlChannel,
	validateProxyRuleLimit func([]domain.ProxyRule) error,
) *runtimeControl {
	return &runtimeControl{
		clientStore:            clientStore,
		sessions:               sessions,
		proxies:                proxies,
		logSessions:            logSessions,
		channel:                channel,
		validateProxyRuleLimit: validateProxyRuleLimit,
	}
}

func (c *runtimeControl) registerClient(cs *ClientSession) {
	c.sessions.add(cs)
	c.persistClientConnectionInfo(cs.ID, cs.RemoteAddr, cs.OS, cs.Arch, cs.Version, 0)
}

func (c *runtimeControl) unregisterClient(cs *ClientSession) {
	c.proxies.stop(cs.ID)
	c.sessions.remove(cs.ID)
	c.logSessions.CleanupClientSessions(cs.ID)
	c.persistClientConnectionInfo(cs.ID, cs.RemoteAddr, cs.OS, cs.Arch, cs.Version, time.Now().Unix())
}

func (c *runtimeControl) persistClientConnectionInfo(clientID, remoteAddr, clientOS, clientArch, clientVersion string, lastOfflineAt int64) {
	if c.clientStore == nil {
		return
	}

	client, err := c.clientStore.GetClient(clientID)
	if err != nil {
		log.Printf("[Server] Load client %s metadata error: %v", clientID, err)
		return
	}

	client.LastRemoteAddr = remoteAddr
	client.LastOS = clientOS
	client.LastArch = clientArch
	client.LastVersion = clientVersion
	client.LastOfflineAt = lastOfflineAt

	if err := c.clientStore.UpdateClient(client); err != nil {
		log.Printf("[Server] Persist client %s metadata error: %v", clientID, err)
	}
}

func (c *runtimeControl) status(clientID string) (clientSessionStatus, bool) {
	return c.sessions.status(clientID)
}

func (c *runtimeControl) allStatus() map[string]clientSessionStatus {
	return c.sessions.allStatus()
}

func (c *runtimeControl) isClientOnline(clientID string) bool {
	return c.sessions.isOnline(clientID)
}

func (c *runtimeControl) isPortAvailable(port int, excludeClientID string) bool {
	return c.proxies.isPortAvailable(port, excludeClientID)
}

func (c *runtimeControl) pushConfig(clientID string) error {
	cs, ok := c.sessions.get(clientID)
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	rules, err := c.clientStore.GetClientRules(clientID)
	if err != nil {
		return err
	}
	if err := c.validateProxyRuleLimit(rules); err != nil {
		return err
	}

	c.proxies.stop(clientID)
	cs.setRules(rules)
	c.proxies.start(cs)

	var failedPorts []string
	for _, rule := range cs.rulesSnapshot() {
		if rule.IsEnabled() && strings.HasPrefix(rule.PortStatus, "failed:") {
			failedPorts = append(failedPorts, fmt.Sprintf("port %d: %s", rule.RemotePort, strings.TrimPrefix(rule.PortStatus, "failed: ")))
		}
	}

	if err := c.channel.sendProxyConfig(cs.Session, cs.rulesSnapshot()); err != nil {
		return err
	}
	if len(failedPorts) > 0 {
		return fmt.Errorf("some ports failed to start: %s", strings.Join(failedPorts, "; "))
	}
	return nil
}

func (c *runtimeControl) disconnectClient(clientID string) error {
	cs, ok := c.sessions.get(clientID)
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}
	return cs.Session.Close()
}

func (c *runtimeControl) restartClient(clientID string) error {
	cs, ok := c.sessions.get(clientID)
	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	if err := c.channel.sendRestart(cs.Session, "server requested restart"); err != nil {
		return err
	}

	time.AfterFunc(100*time.Millisecond, func() {
		_ = cs.Session.Close()
	})

	log.Printf("[Server] Restart initiated for client %s", clientID)
	return nil
}

func (c *runtimeControl) sendUpdate(clientID, downloadURL string) error {
	cs, ok := c.sessions.get(clientID)
	if !ok {
		return fmt.Errorf("client %s not found or not online", clientID)
	}

	if err := c.channel.sendUpdate(cs.Session, downloadURL); err != nil {
		return err
	}

	log.Printf("[Server] Update command sent to client %s: %s", clientID, downloadURL)
	return nil
}
