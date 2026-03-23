package runtime

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gotunnel/pkg/security"
)

type listenerRuntime struct {
	connSem  chan struct{}
	shutdown chan struct{}
	listener net.Listener
	wg       sync.WaitGroup
}

func newListenerRuntime(maxConnections int) *listenerRuntime {
	return &listenerRuntime{
		connSem:  make(chan struct{}, maxConnections),
		shutdown: make(chan struct{}),
	}
}

func (r *listenerRuntime) run(bindAddr string, bindPort int, tlsConfig *tls.Config, handler func(net.Conn)) error {
	addr := fmt.Sprintf("%s:%d", bindAddr, bindPort)

	ln, err := r.listen(addr, tlsConfig)
	if err != nil {
		return err
	}
	r.listener = ln

	for {
		select {
		case <-r.shutdown:
			log.Printf("[Server] Shutdown signal received, stopping accept loop")
			_ = ln.Close()
			return nil
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-r.shutdown:
				return nil
			default:
				log.Printf("[Server] Accept error: %v", err)
				continue
			}
		}

		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.handleAcceptedConn(conn, handler)
		}()
	}
}

func (r *listenerRuntime) shutdownGracefully(timeout time.Duration, disconnectAll func()) error {
	log.Printf("[Server] Initiating graceful shutdown...")
	close(r.shutdown)

	if r.listener != nil {
		_ = r.listener.Close()
	}
	if disconnectAll != nil {
		disconnectAll()
	}

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[Server] All connections closed gracefully")
		return nil
	case <-time.After(timeout):
		log.Printf("[Server] Shutdown timeout, forcing close")
		return fmt.Errorf("shutdown timeout")
	}
}

func (r *listenerRuntime) listen(addr string, tlsConfig *tls.Config) (net.Listener, error) {
	if tlsConfig != nil {
		ln, err := tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to listen TLS on %s: %v", addr, err)
		}
		log.Printf("[Server] TLS listening on %s", addr)
		return ln, nil
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	log.Printf("[Server] Listening on %s (no TLS)", addr)
	return ln, nil
}

func (r *listenerRuntime) handleAcceptedConn(conn net.Conn, handler func(net.Conn)) {
	clientIP := conn.RemoteAddr().String()

	select {
	case r.connSem <- struct{}{}:
		defer func() { <-r.connSem }()
	default:
		security.LogConnRejected(clientIP, "max connections reached")
		_ = conn.Close()
		return
	}

	defer conn.Close()
	handler(conn)
}
