package tunnel

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gotunnel/internal/server/domain"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/proxy"
	"github.com/gotunnel/pkg/relay"
)

// startProxyListeners 启动代理监听
func (s *Server) startProxyListeners(cs *ClientSession) {
	s.proxies.start(cs)
}

func (m *proxyManager) start(cs *ClientSession) {
	rules := cs.rulesSnapshot()

	for i := range rules {
		rule := &rules[i]
		if !rule.IsEnabled() {
			continue
		}

		ruleType := rule.Type
		if ruleType == "" {
			ruleType = "tcp"
		}

		if ruleType == "udp" {
			m.startUDPListener(cs, rule)
			continue
		}

		if err := m.portManager.Reserve(rule.RemotePort, cs.ID); err != nil {
			log.Printf("[Server] Port %d error: %v", rule.RemotePort, err)
			rule.PortStatus = "failed: " + err.Error()
			continue
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", rule.RemotePort))
		if err != nil {
			log.Printf("[Server] Listen %d error: %v", rule.RemotePort, err)
			m.portManager.Release(rule.RemotePort)
			rule.PortStatus = "failed: " + err.Error()
			continue
		}

		rule.PortStatus = "listening"
		m.bindTCPListener(cs.ID, rule.RemotePort, ln)

		switch ruleType {
		case "socks5":
			log.Printf("[Server] SOCKS5 proxy %s on :%d", rule.Name, rule.RemotePort)
			go m.acceptProxyServerConns(cs, ln, *rule)
		case "http", "https":
			log.Printf("[Server] HTTP proxy %s on :%d", rule.Name, rule.RemotePort)
			go m.acceptProxyServerConns(cs, ln, *rule)
		case "websocket":
			log.Printf("[Server] Websocket proxy %s on :%d", rule.Name, rule.RemotePort)
			go m.acceptWebsocketConns(cs, ln, *rule)
		default:
			log.Printf("[Server] TCP proxy %s: :%d -> %s:%d", rule.Name, rule.RemotePort, rule.LocalIP, rule.LocalPort)
			go m.acceptProxyConns(cs, ln, *rule)
		}
	}

	cs.setRules(rules)
}

// acceptProxyConns 接受代理连接
func (m *proxyManager) acceptProxyConns(cs *ClientSession, ln net.Listener, rule domain.ProxyRule) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go m.handleProxyConn(cs, conn, rule)
	}
}

// acceptProxyServerConns 接受 SOCKS5/HTTP 代理连接
func (m *proxyManager) acceptProxyServerConns(cs *ClientSession, ln net.Listener, rule domain.ProxyRule) {
	dialer := proxy.NewTunnelDialer(cs.Session)

	username := ""
	password := ""
	if rule.AuthEnabled {
		username = rule.AuthUsername
		password = rule.AuthPassword
	}
	proxyServer := proxy.NewServer(rule.Type, dialer, m.recordTraffic, username, password)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go proxyServer.HandleConn(conn)
	}
}

// handleProxyConn 处理代理连接
func (m *proxyManager) handleProxyConn(cs *ClientSession, conn net.Conn, rule domain.ProxyRule) {
	defer conn.Close()

	stream, err := cs.Session.Open()
	if err != nil {
		log.Printf("[Server] Open stream error: %v", err)
		return
	}
	defer stream.Close()

	if err := m.requestProxyOpen(stream, rule.RemotePort); err != nil {
		log.Printf("[Server] Proxy %s open failed for client %s on port %d: %v", rule.Name, cs.ID, rule.RemotePort, err)
		return
	}

	relay.RelayWithStats(conn, stream, m.recordTraffic)
}

// startUDPListener 启动 UDP 监听
func (m *proxyManager) startUDPListener(cs *ClientSession, rule *domain.ProxyRule) {
	if err := m.portManager.Reserve(rule.RemotePort, cs.ID); err != nil {
		log.Printf("[Server] UDP port %d error: %v", rule.RemotePort, err)
		rule.PortStatus = "failed: " + err.Error()
		return
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", rule.RemotePort))
	if err != nil {
		log.Printf("[Server] UDP resolve error: %v", err)
		m.portManager.Release(rule.RemotePort)
		rule.PortStatus = "failed: " + err.Error()
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("[Server] UDP listen %d error: %v", rule.RemotePort, err)
		m.portManager.Release(rule.RemotePort)
		rule.PortStatus = "failed: " + err.Error()
		return
	}

	rule.PortStatus = "listening"
	m.bindUDPConn(cs.ID, rule.RemotePort, conn)

	log.Printf("[Server] UDP proxy %s: :%d -> %s:%d", rule.Name, rule.RemotePort, rule.LocalIP, rule.LocalPort)

	go m.handleUDPConn(cs, conn, *rule)
}

// handleUDPConn 处理 UDP 连接
func (m *proxyManager) handleUDPConn(cs *ClientSession, conn *net.UDPConn, rule domain.ProxyRule) {
	buf := make([]byte, udpBufferSize)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		packet := protocol.UDPPacket{
			RemotePort: rule.RemotePort,
			ClientAddr: clientAddr.String(),
			Data:       buf[:n],
		}

		go m.sendUDPPacket(cs, conn, clientAddr, packet)
	}
}

// sendUDPPacket 发送 UDP 数据包到客户端
func (m *proxyManager) sendUDPPacket(cs *ClientSession, conn *net.UDPConn, clientAddr *net.UDPAddr, packet protocol.UDPPacket) {
	stream, err := cs.Session.Open()
	if err != nil {
		return
	}
	defer stream.Close()

	msg, err := protocol.NewMessage(protocol.MsgTypeUDPData, packet)
	if err != nil {
		return
	}

	if err := withStreamDeadline(stream, m.clientResponseTimeout(), func() error {
		if err := protocol.WriteMessage(stream, msg); err != nil {
			return err
		}

		m.recordTraffic(int64(len(packet.Data)), 0)

		respMsg, err := protocol.ReadMessage(stream)
		if err != nil {
			return err
		}

		if respMsg.Type != protocol.MsgTypeUDPData {
			return nil
		}

		var respPacket protocol.UDPPacket
		if err := respMsg.ParsePayload(&respPacket); err != nil {
			return err
		}
		if _, err := conn.WriteToUDP(respPacket.Data, clientAddr); err != nil {
			return err
		}
		m.recordTraffic(0, int64(len(respPacket.Data)))
		return nil
	}); err != nil {
		log.Printf("[Server] UDP proxy %s response failed on port %d: %v", cs.ID, packet.RemotePort, err)
	}
}

// checkHTTPBasicAuth 检查 HTTP Basic Auth
// 返回 (认证成功, 已读取的数据)
func (m *proxyManager) checkHTTPBasicAuth(conn net.Conn, username, password string) (bool, []byte) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	buf := make([]byte, 8192)
	n, err := conn.Read(buf)
	if err != nil {
		return false, nil
	}

	data := buf[:n]
	request := string(data)

	authHeader := ""
	lines := strings.Split(request, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "authorization:") {
			authHeader = strings.TrimSpace(line[14:])
			break
		}
	}

	if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
		m.sendHTTPUnauthorized(conn)
		return false, nil
	}

	encoded := authHeader[6:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		m.sendHTTPUnauthorized(conn)
		return false, nil
	}

	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		m.sendHTTPUnauthorized(conn)
		return false, nil
	}

	if parts[0] != username || parts[1] != password {
		m.sendHTTPUnauthorized(conn)
		return false, nil
	}

	return true, data
}

// sendHTTPUnauthorized 发送 401 未授权响应
func (m *proxyManager) sendHTTPUnauthorized(conn net.Conn) {
	response := "HTTP/1.1 401 Unauthorized\r\n" +
		"WWW-Authenticate: Basic realm=\"GoTunnel Plugin\"\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 12\r\n" +
		"\r\n" +
		"Unauthorized"
	conn.Write([]byte(response))
}
