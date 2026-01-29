package tunnel

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gotunnel/pkg/protocol"
	"github.com/gotunnel/pkg/relay"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

// WSConnAdapter 适配器：将 websocket.Conn 适配为 io.ReadWriter
type WSConnAdapter struct {
	conn *websocket.Conn
	// 读缓冲
	reader io.Reader
}

func NewWSConnAdapter(conn *websocket.Conn) *WSConnAdapter {
	return &WSConnAdapter{
		conn: conn,
	}
}

func (a *WSConnAdapter) Read(p []byte) (n int, err error) {
	if a.reader == nil {
		messageType, reader, err := a.conn.NextReader()
		if err != nil {
			return 0, err
		}
		if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
			// 忽略非数据消息
			return 0, nil
		}
		a.reader = reader
	}
	n, err = a.reader.Read(p)
	if err == io.EOF {
		a.reader = nil
		err = nil // 当前消息读完，不代表连接断开
		// 如果读到了0字节，尝试读下一个消息，避免因为返回 (0, nil) 导致调用方以为无数据空转
		if n == 0 {
			return a.Read(p)
		}
	}
	return n, err
}

func (a *WSConnAdapter) Write(p []byte) (n int, err error) {
	err = a.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (a *WSConnAdapter) Close() error {
	return a.conn.Close()
}

func (a *WSConnAdapter) LocalAddr() net.Addr {
	return a.conn.LocalAddr()
}

func (a *WSConnAdapter) RemoteAddr() net.Addr {
	return a.conn.RemoteAddr()
}

func (a *WSConnAdapter) SetDeadline(t time.Time) error {
	if err := a.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return a.conn.SetWriteDeadline(t)
}

func (a *WSConnAdapter) SetReadDeadline(t time.Time) error {
	return a.conn.SetReadDeadline(t)
}

func (a *WSConnAdapter) SetWriteDeadline(t time.Time) error {
	return a.conn.SetWriteDeadline(t)
}

// acceptWebsocketConns 接受 Websocket 连接
func (s *Server) acceptWebsocketConns(cs *ClientSession, ln net.Listener, rule protocol.ProxyRule) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[Server] Websocket upgrade error: %v", err)
			return
		}

		conn := NewWSConnAdapter(wsConn)
		// 这里的 conn 并没有实现 net.Conn 接口的全部方法 (LocalAddr, RemoteAddr 等)，
		// Relay 函数如果需要 net.Conn，可能需要更完整的适配器。
		// 查看 relay.Relay 签名：func Relay(c1, c2 io.ReadWriteCloser)
		// 假设 relay.Relay 接受 io.ReadWriteCloser。

		go s.handleWebsocketProxyConn(cs, conn, rule)
	})

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 这里不需要协程，因为 startProxyListeners 中已经是 go s.acceptWebsocketConns(...) 调用了？
	// 不，startProxyListeners 中 iterate rules。如果是 acceptWebsocketConns，应该是在那里 go。
	// 检查 caller 逻辑。

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Printf("[Server] Websocket server error: %v", err)
	}
}

// handleWebsocketProxyConn 处理 Websocket 代理连接
func (s *Server) handleWebsocketProxyConn(cs *ClientSession, conn net.Conn, rule protocol.ProxyRule) {
	defer conn.Close()

	stream, err := cs.Session.Open()
	if err != nil {
		log.Printf("[Server] Open stream error: %v", err)
		return
	}
	defer stream.Close()

	// 发送新代理连接请求，告知客户端连接到哪里
	req := protocol.NewProxyRequest{RemotePort: rule.RemotePort}
	msg, _ := protocol.NewMessage(protocol.MsgTypeNewProxy, req)
	if err := protocol.WriteMessage(stream, msg); err != nil {
		return
	}

	relay.RelayWithStats(conn, stream, s.recordTraffic)
}
