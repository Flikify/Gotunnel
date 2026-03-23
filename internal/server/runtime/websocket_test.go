package runtime

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWSConnAdapter(t *testing.T) {
	// 1. 设置测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer c.Close()

		adapter := NewWSConnAdapter(c)
		defer adapter.Close()

		// Echo server
		buf := make([]byte, 1024)
		for {
			n, err := adapter.Read(buf)
			if err != nil {
				if err != io.EOF {
					// websocket close might cause normal error locally
				}
				break
			}
			_, err = adapter.Write(buf[:n])
			if err != nil {
				t.Errorf("write error: %v", err)
				break
			}
		}
	}))
	defer server.Close()

	// 2. 客户端连接
	u := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	// 3. 发送数据
	message := []byte("hello websocket")
	err = ws.WriteMessage(websocket.BinaryMessage, message)
	if err != nil {
		t.Fatalf("write message error: %v", err)
	}

	// 4. 接收响应
	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read message error: %v", err)
	}

	if !bytes.Equal(message, p) {
		t.Errorf("expected %s, got %s", message, p)
	}
}

func TestWSConnAdapter_ReadMultiFrame(t *testing.T) {
	// 测试多次 Read 调用读取一个 frame，或者一个 Read 读取多个 frame (net.Conn 语义)
	// WSConnAdapter 实现是 Read 对应 NextReader，如果 buffer 小，可能一部分一部分读。

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		adapter := NewWSConnAdapter(c)

		// 只要收到数据就这就验证通过
		buf := make([]byte, 10)
		n, err := adapter.Read(buf)
		if err != nil {
			t.Errorf("read error: %v", err)
		}
		if n != 5 { // "hello"
			t.Errorf("expected 5 bytes, got %d", n)
		}

		// 读剩下的 "world"
		n, err = adapter.Read(buf)
		if err != nil {
			t.Errorf("read 2 error: %v", err)
		}
		if n != 5 {
			t.Errorf("expected 5 bytes, got %d", n)
		}
	}))
	defer server.Close()

	u := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	// 发送两个 BinaryMessage
	ws.WriteMessage(websocket.BinaryMessage, []byte("hello"))
	ws.WriteMessage(websocket.BinaryMessage, []byte("world"))

	time.Sleep(100 * time.Millisecond)
}
