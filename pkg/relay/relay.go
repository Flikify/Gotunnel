package relay

import (
	"net"
	"sync"
)

// Relay 双向数据转发
func Relay(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copy := func(dst, src net.Conn) {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				dst.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}

	go copy(c1, c2)
	go copy(c2, c1)
	wg.Wait()
}
