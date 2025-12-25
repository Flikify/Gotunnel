package proxy

import (
	"log"
	"net"
)

// Server 代理服务器
type Server struct {
	socks5   *SOCKS5Server
	http     *HTTPServer
	listener net.Listener
	typ      string
}

// NewServer 创建代理服务器
func NewServer(typ string, dialer Dialer) *Server {
	return &Server{
		socks5: NewSOCKS5Server(dialer),
		http:   NewHTTPServer(dialer),
		typ:    typ,
	}
}

// Run 启动代理服务
func (s *Server) Run(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	log.Printf("[Proxy] %s listening on %s", s.typ, addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.HandleConn(conn)
	}
}

func (s *Server) HandleConn(conn net.Conn) {
	var err error
	switch s.typ {
	case "socks5":
		err = s.socks5.HandleConn(conn)
	case "http":
		err = s.http.HandleConn(conn)
	}
	if err != nil {
		log.Printf("[Proxy] Error: %v", err)
	}
}

// Close 关闭服务
func (s *Server) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
