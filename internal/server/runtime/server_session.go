package runtime

import (
	"errors"
	"log"
	"net"

	"github.com/gotunnel/pkg/security"
)

// Run 启动服务端
func (s *Server) Run() error {
	return s.listenerLoop.run(s.bindAddr, s.bindPort, s.tlsConfig, s.handleConnection)
}

// handleConnection 处理客户端连接
func (s *Server) handleConnection(conn net.Conn) {
	admitted, err := s.admission.admit(conn)
	if err != nil {
		var rejection *admissionRejectionError
		if errors.As(err, &rejection) {
			_ = s.sendAuthResponse(conn, false, rejection.message, "")
			return
		}
		log.Printf("[Server] Admission error: %v", err)
		return
	}

	if err := s.sendAuthResponse(conn, true, "ok", admitted.ID); err != nil {
		return
	}

	security.LogAuthSuccess(conn.RemoteAddr().String(), admitted.ID)
	s.lifecycle.run(conn, admitted)
}
