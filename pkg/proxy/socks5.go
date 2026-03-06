package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/gotunnel/pkg/relay"
)

const (
	socks5Version = 0x05
	noAuth        = 0x00
	cmdConnect    = 0x01
	atypIPv4      = 0x01
	atypDomain    = 0x03
	atypIPv6      = 0x04
)

// SOCKS5Server SOCKS5 代理服务
type SOCKS5Server struct {
	dialer  Dialer
	onStats func(in, out int64) // 流量统计回调
}

// Dialer 连接拨号器接口
type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// NewSOCKS5Server 创建 SOCKS5 服务
func NewSOCKS5Server(dialer Dialer, onStats func(in, out int64)) *SOCKS5Server {
	return &SOCKS5Server{dialer: dialer, onStats: onStats}
}

// HandleConn 处理 SOCKS5 连接
func (s *SOCKS5Server) HandleConn(conn net.Conn) error {
	defer conn.Close()

	// 握手阶段
	if err := s.handshake(conn); err != nil {
		return err
	}

	// 获取请求
	target, err := s.readRequest(conn)
	if err != nil {
		return err
	}

	// 连接目标
	remote, err := s.dialer.Dial("tcp", target)
	if err != nil {
		s.sendReply(conn, 0x05) // Connection refused
		return err
	}
	defer remote.Close()

	// 发送成功响应
	if err := s.sendReply(conn, 0x00); err != nil {
		return err
	}

	// 双向转发 (带流量统计)
	relay.RelayWithStats(conn, remote, s.onStats)

	return nil
}

// handshake 处理握手
func (s *SOCKS5Server) handshake(conn net.Conn) error {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	if buf[0] != socks5Version {
		return errors.New("unsupported SOCKS version")
	}

	nmethods := int(buf[1])
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	// 响应：使用无认证
	_, err := conn.Write([]byte{socks5Version, noAuth})
	return err
}

// readRequest 读取请求
func (s *SOCKS5Server) readRequest(conn net.Conn) (string, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	if buf[0] != socks5Version || buf[1] != cmdConnect {
		return "", errors.New("unsupported command")
	}

	var host string
	switch buf[3] {
	case atypIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case atypDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", err
		}
		host = string(domain)
	case atypIPv6:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	default:
		return "", errors.New("unsupported address type")
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBuf)

	return fmt.Sprintf("%s:%d", host, port), nil
}

// sendReply 发送响应
func (s *SOCKS5Server) sendReply(conn net.Conn, rep byte) error {
	// VER REP RSV ATYP BND.ADDR BND.PORT
	reply := []byte{socks5Version, rep, 0x00, atypIPv4, 0, 0, 0, 0, 0, 0}
	_, err := conn.Write(reply)
	return err
}
