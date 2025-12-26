package relay

import (
	"io"
	"net"
	"sync"
)

const bufferSize = 32 * 1024

// Relay 双向数据转发
func Relay(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copyConn := func(dst, src net.Conn) {
		defer wg.Done()
		buf := make([]byte, bufferSize)
		_, _ = io.CopyBuffer(dst, src, buf)
		// 关闭写端，通知对方数据传输完成
		if tc, ok := dst.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}

	go copyConn(c1, c2)
	go copyConn(c2, c1)
	wg.Wait()
}
