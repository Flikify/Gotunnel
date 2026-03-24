package runtime

import (
	"crypto/tls"
	"regexp"
	"sync"
	"time"

	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/observability"
)

// 服务端常量
const (
	authTimeout    = 10 * time.Second
	udpBufferSize  = 65535
	maxConnections = 10000 // 最大连接数
)

// 客户端 ID 验证正则
var clientIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// isValidClientID 验证客户端 ID 格式
func isValidClientID(id string) bool {
	return clientIDRegex.MatchString(id)
}

// Server 隧道服务端
type Server struct {
	trafficStore db.TrafficStore // 流量存储
	bindAddr     string
	bindPort     int
	configMu     sync.RWMutex
	heartbeatSec int
	hbTimeoutSec int
	maxProxies   int
	respTimeout  int
	sessions     *clientSessionRegistry
	proxies      *proxyManager
	admission    *clientAdmission
	control      *runtimeControl
	channel      *controlChannel
	lifecycle    *sessionLifecycle
	listenerLoop *listenerRuntime
	tlsConfig    *tls.Config
	logSessions  *LogSessionManager // 日志会话管理器
	diagStore    *observability.DiagnosticStore
	eventStore   db.OperationalEventStore
	ingestor     *eventIngestor
}

// NewServer 创建服务端
func NewServer(cs db.ClientStore, bindAddr string, bindPort int, token string, heartbeat, hbTimeout int) *Server {
	s := &Server{
		bindAddr:     bindAddr,
		bindPort:     bindPort,
		sessions:     newClientSessionRegistry(),
		listenerLoop: newListenerRuntime(maxConnections),
		logSessions:  NewLogSessionManager(),
	}
	s.channel = newControlChannel(s.clientResponseTimeout)
	s.proxies = newProxyManager(s.recordTraffic, s.clientResponseTimeout, s.requestProxyOpen, s.emitServerEvent)
	s.admission = newClientAdmission(token, cs)
	s.control = newRuntimeControl(cs, s.sessions, s.proxies, s.logSessions, s.channel, s.validateProxyRuleLimit)
	s.lifecycle = newSessionLifecycle(
		s.validateProxyRuleLimit,
		s.control.registerClient,
		s.control.unregisterClient,
		s.startProxyListeners,
		s.emitServerEvent,
		s.channel,
		s.handleClientInitiatedStreams,
		s.runtimeConfig,
	)
	s.ApplyRuntimeConfig(heartbeat, hbTimeout, 0, 15)
	return s
}
