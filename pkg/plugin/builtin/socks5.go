package builtin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/gotunnel/pkg/plugin"
)

const (
	socks5Version = 0x05
	noAuth        = 0x00
	cmdConnect    = 0x01
	atypIPv4      = 0x01
	atypDomain    = 0x03
	atypIPv6      = 0x04
)

// SOCKS5Plugin 将现有 SOCKS5 实现封装为 plugin
type SOCKS5Plugin struct {
	config map[string]string
}

// NewSOCKS5Plugin 创建 SOCKS5 plugin
func NewSOCKS5Plugin() *SOCKS5Plugin {
	return &SOCKS5Plugin{}
}

// Metadata 返回 plugin 信息
func (p *SOCKS5Plugin) Metadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:        "socks5",
		Version:     "1.0.0",
		Type:        plugin.PluginTypeProxy,
		Source:      plugin.PluginSourceBuiltin,
		Description: "SOCKS5 proxy protocol handler (official plugin)",
		Author:      "GoTunnel",
		Capabilities: []string{
			"dial", "read", "write", "close",
		},
	}
}

// Init 初始化 plugin
func (p *SOCKS5Plugin) Init(config map[string]string) error {
	p.config = config
	return nil
}

// HandleConn 处理 SOCKS5 连接
func (p *SOCKS5Plugin) HandleConn(conn net.Conn, dialer plugin.Dialer) error {
	defer conn.Close()

	// 握手阶段
	if err := p.handshake(conn); err != nil {
		return err
	}

	// 获取请求
	target, err := p.readRequest(conn)
	if err != nil {
		return err
	}

	// 连接目标
	remote, err := dialer.Dial("tcp", target)
	if err != nil {
		p.sendReply(conn, 0x05) // Connection refused
		return err
	}
	defer remote.Close()

	// 发送成功响应
	if err := p.sendReply(conn, 0x00); err != nil {
		return err
	}

	// 双向转发
	go io.Copy(remote, conn)
	io.Copy(conn, remote)

	return nil
}

// Close 释放资源
func (p *SOCKS5Plugin) Close() error {
	return nil
}

// handshake 处理握手
func (p *SOCKS5Plugin) handshake(conn net.Conn) error {
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
func (p *SOCKS5Plugin) readRequest(conn net.Conn) (string, error) {
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
func (p *SOCKS5Plugin) sendReply(conn net.Conn, rep byte) error {
	reply := []byte{socks5Version, rep, 0x00, atypIPv4, 0, 0, 0, 0, 0, 0}
	_, err := conn.Write(reply)
	return err
}
