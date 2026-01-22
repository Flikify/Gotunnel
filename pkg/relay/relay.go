package relay

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
)

const bufferSize = 32 * 1024

// TrafficStats 流量统计
type TrafficStats struct {
	Inbound  int64
	Outbound int64
}

// Relay 双向数据转发
func Relay(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copyConn := func(dst, src net.Conn) {
		defer wg.Done()
		buf := make([]byte, bufferSize)
		_, _ = io.CopyBuffer(dst, src, buf)
		if tc, ok := dst.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}

	go copyConn(c1, c2)
	go copyConn(c2, c1)
	wg.Wait()
}

// RelayWithStats 带流量统计的双向数据转发
func RelayWithStats(c1, c2 net.Conn, onStats func(in, out int64)) {
	var wg sync.WaitGroup
	var inbound, outbound int64
	wg.Add(2)

	copyWithCount := func(dst, src net.Conn, counter *int64) {
		defer wg.Done()
		buf := make([]byte, bufferSize)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				atomic.AddInt64(counter, int64(n))
				dst.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		if tc, ok := dst.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}

	go copyWithCount(c1, c2, &inbound)
	go copyWithCount(c2, c1, &outbound)
	wg.Wait()

	if onStats != nil {
		onStats(inbound, outbound)
	}
}
