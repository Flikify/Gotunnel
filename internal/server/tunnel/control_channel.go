package tunnel

import (
	"fmt"
	"net"
	"time"

	"github.com/gotunnel/internal/server/domain"
	"github.com/gotunnel/pkg/protocol"
	"github.com/hashicorp/yamux"
)

type controlChannel struct {
	clientResponseTimeout func() time.Duration
}

func newControlChannel(clientResponseTimeout func() time.Duration) *controlChannel {
	return &controlChannel{
		clientResponseTimeout: clientResponseTimeout,
	}
}

func (c *controlChannel) sendAuthResponse(conn net.Conn, success bool, message, clientID string) error {
	resp := protocol.AuthResponse{Success: success, Message: message, ClientID: clientID}
	msg, err := protocol.NewMessage(protocol.MsgTypeAuthResp, resp)
	if err != nil {
		return err
	}
	return protocol.WriteMessage(conn, msg)
}

func (c *controlChannel) requestProxyOpen(stream net.Conn, remotePort int) error {
	msg, err := protocol.NewMessage(protocol.MsgTypeNewProxy, protocol.NewProxyRequest{RemotePort: remotePort})
	if err != nil {
		return err
	}

	return withStreamDeadline(stream, c.clientResponseTimeout(), func() error {
		if err := protocol.WriteMessage(stream, msg); err != nil {
			return err
		}

		resp, err := protocol.ReadMessage(stream)
		if err != nil {
			return fmt.Errorf("wait proxy result: %w", err)
		}
		if resp.Type != protocol.MsgTypeProxyResult {
			return fmt.Errorf("unexpected proxy result type: %d", resp.Type)
		}

		var result protocol.ProxyConnectResult
		if err := resp.ParsePayload(&result); err != nil {
			return err
		}
		if !result.Success {
			if result.Message == "" {
				result.Message = "client rejected proxy connection"
			}
			return fmt.Errorf("%s", result.Message)
		}
		return nil
	})
}

func (c *controlChannel) sendProxyConfig(session *yamux.Session, rules []domain.ProxyRule) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	cfg := protocol.ProxyConfig{Rules: toProtocolRules(rules)}
	msg, err := protocol.NewMessage(protocol.MsgTypeProxyConfig, cfg)
	if err != nil {
		return err
	}
	return withStreamDeadline(stream, c.clientResponseTimeout(), func() error {
		if err := protocol.WriteMessage(stream, msg); err != nil {
			return err
		}

		ack, err := protocol.ReadMessage(stream)
		if err != nil {
			return fmt.Errorf("wait config ack: %w", err)
		}
		if ack.Type != protocol.MsgTypeProxyReady {
			return fmt.Errorf("unexpected ack type: %d", ack.Type)
		}

		return nil
	})
}

func (c *controlChannel) sendHeartbeat(cs *ClientSession) bool {
	stream, err := cs.Session.Open()
	if err != nil {
		return false
	}
	defer stream.Close()

	msg := &protocol.Message{Type: protocol.MsgTypeHeartbeat}
	var ok bool
	if err := withStreamDeadline(stream, c.clientResponseTimeout(), func() error {
		if err := protocol.WriteMessage(stream, msg); err != nil {
			return err
		}

		resp, err := protocol.ReadMessage(stream)
		if err != nil {
			return err
		}

		ok = resp.Type == protocol.MsgTypeHeartbeatAck
		return nil
	}); err != nil {
		return false
	}

	return ok
}

func (c *controlChannel) sendRestart(session *yamux.Session, reason string) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.ClientRestartRequest{Reason: reason}
	msg, err := protocol.NewMessage(protocol.MsgTypeClientRestart, req)
	if err != nil {
		return err
	}
	return protocol.WriteMessage(stream, msg)
}

func (c *controlChannel) sendUpdate(session *yamux.Session, downloadURL string) error {
	stream, err := session.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	req := protocol.UpdateDownloadRequest{DownloadURL: downloadURL}
	msg, err := protocol.NewMessage(protocol.MsgTypeUpdateDownload, req)
	if err != nil {
		return err
	}
	return protocol.WriteMessage(stream, msg)
}
