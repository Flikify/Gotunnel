package builtin

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gotunnel/pkg/plugin"
)

// HTTPPlugin 将现有 HTTP 代理实现封装为 plugin
type HTTPPlugin struct {
	config map[string]string
}

// NewHTTPPlugin 创建 HTTP plugin
func NewHTTPPlugin() *HTTPPlugin {
	return &HTTPPlugin{}
}

// Metadata 返回 plugin 信息
func (p *HTTPPlugin) Metadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:        "http",
		Version:     "1.0.0",
		Type:        plugin.PluginTypeProxy,
		Source:      plugin.PluginSourceBuiltin,
		Description: "HTTP/HTTPS proxy protocol handler",
		Author:      "GoTunnel",
		Capabilities: []string{
			"dial", "read", "write", "close",
		},
	}
}

// Init 初始化 plugin
func (p *HTTPPlugin) Init(config map[string]string) error {
	p.config = config
	return nil
}

// HandleConn 处理 HTTP 代理连接
func (p *HTTPPlugin) HandleConn(conn net.Conn, dialer plugin.Dialer) error {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}

	if req.Method == http.MethodConnect {
		return p.handleConnect(conn, req, dialer)
	}
	return p.handleHTTP(conn, req, dialer)
}

// Close 释放资源
func (p *HTTPPlugin) Close() error {
	return nil
}

// handleConnect 处理 CONNECT 方法 (HTTPS)
func (p *HTTPPlugin) handleConnect(conn net.Conn, req *http.Request, dialer plugin.Dialer) error {
	target := req.Host
	if !strings.Contains(target, ":") {
		target = target + ":443"
	}

	remote, err := dialer.Dial("tcp", target)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return err
	}
	defer remote.Close()

	conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go io.Copy(remote, conn)
	io.Copy(conn, remote)
	return nil
}

// handleHTTP 处理普通 HTTP 请求
func (p *HTTPPlugin) handleHTTP(conn net.Conn, req *http.Request, dialer plugin.Dialer) error {
	target := req.Host
	if !strings.Contains(target, ":") {
		target = target + ":80"
	}

	remote, err := dialer.Dial("tcp", target)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return err
	}
	defer remote.Close()

	// 修改请求路径为相对路径
	req.URL.Scheme = ""
	req.URL.Host = ""
	req.RequestURI = req.URL.Path
	if req.URL.RawQuery != "" {
		req.RequestURI += "?" + req.URL.RawQuery
	}

	// 发送请求到目标
	if err := req.Write(remote); err != nil {
		return err
	}

	// 转发响应
	_, err = io.Copy(conn, remote)
	return err
}
