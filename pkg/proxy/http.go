package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
)

// HTTPServer HTTP 代理服务
type HTTPServer struct {
	dialer Dialer
}

// NewHTTPServer 创建 HTTP 代理服务
func NewHTTPServer(dialer Dialer) *HTTPServer {
	return &HTTPServer{dialer: dialer}
}

// HandleConn 处理 HTTP 代理连接
func (h *HTTPServer) HandleConn(conn net.Conn) error {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}

	if req.Method == http.MethodConnect {
		return h.handleConnect(conn, req)
	}
	return h.handleHTTP(conn, req, reader)
}

// handleConnect 处理 CONNECT 方法 (HTTPS)
func (h *HTTPServer) handleConnect(conn net.Conn, req *http.Request) error {
	target := req.Host
	if !strings.Contains(target, ":") {
		target = target + ":443"
	}

	remote, err := h.dialer.Dial("tcp", target)
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
func (h *HTTPServer) handleHTTP(conn net.Conn, req *http.Request, reader *bufio.Reader) error {
	target := req.Host
	if !strings.Contains(target, ":") {
		target = target + ":80"
	}

	remote, err := h.dialer.Dial("tcp", target)
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
