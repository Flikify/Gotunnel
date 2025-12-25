package proxy

import (
	"errors"
	"net"

	"github.com/gotunnel/pkg/protocol"
	"github.com/hashicorp/yamux"
)

// TunnelDialer 通过隧道连接的拨号器
type TunnelDialer struct {
	session *yamux.Session
}

// NewTunnelDialer 创建隧道拨号器
func NewTunnelDialer(session *yamux.Session) *TunnelDialer {
	return &TunnelDialer{session: session}
}

// Dial 通过隧道建立连接
func (d *TunnelDialer) Dial(network, address string) (net.Conn, error) {
	stream, err := d.session.Open()
	if err != nil {
		return nil, err
	}

	// 发送代理连接请求
	req := protocol.ProxyConnectRequest{Target: address}
	msg, err := protocol.NewMessage(protocol.MsgTypeProxyConnect, req)
	if err != nil {
		stream.Close()
		return nil, err
	}

	if err := protocol.WriteMessage(stream, msg); err != nil {
		stream.Close()
		return nil, err
	}

	// 读取连接结果
	respMsg, err := protocol.ReadMessage(stream)
	if err != nil {
		stream.Close()
		return nil, err
	}

	if respMsg.Type != protocol.MsgTypeProxyResult {
		stream.Close()
		return nil, errors.New("unexpected response type")
	}

	var result protocol.ProxyConnectResult
	if err := respMsg.ParsePayload(&result); err != nil {
		stream.Close()
		return nil, err
	}

	if !result.Success {
		stream.Close()
		return nil, errors.New(result.Message)
	}

	return stream, nil
}
