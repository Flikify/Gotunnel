package service

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gotunnel/pkg/protocol"
)

type RemoteControlService interface {
	OpenSession(clientID string, start protocol.RemoteControlStart) (*RemoteControlSession, error)
}

type RemoteControlSession struct {
	ClientID string
	Stream   net.Conn
	Ready    protocol.RemoteControlReady

	release func()
	once    sync.Once
}

func (s *RemoteControlSession) Stop(reason string) {
	s.once.Do(func() {
		if s.Stream != nil {
			_ = withStreamDeadline(s.Stream, 250*time.Millisecond, func() error {
				msg, err := protocol.NewMessage(protocol.MsgTypeRemoteControlStop, protocol.RemoteControlStop{Reason: reason})
				if err != nil {
					return err
				}
				return protocol.WriteMessage(s.Stream, msg)
			})
			_ = s.Stream.Close()
		}
		if s.release != nil {
			s.release()
		}
	})
}

type remoteControlService struct {
	runtime RemoteOpsRuntime

	mu     sync.Mutex
	active map[string]struct{}
}

func NewRemoteControlService(runtime RemoteOpsRuntime) RemoteControlService {
	return &remoteControlService{
		runtime: runtime,
		active:  make(map[string]struct{}),
	}
}

func (s *remoteControlService) OpenSession(clientID string, start protocol.RemoteControlStart) (*RemoteControlSession, error) {
	if !s.runtime.IsClientOnline(clientID) {
		return nil, ErrClientNotOnline
	}
	if !s.tryAcquire(clientID) {
		return nil, ErrRemoteControlSessionActive
	}

	stream, err := s.runtime.OpenClientStream(clientID)
	if err != nil {
		s.release(clientID)
		return nil, err
	}

	timeout := s.runtime.ClientResponseTimeout()
	if timeout > 0 {
		if err := stream.SetDeadline(now().Add(timeout)); err != nil {
			_ = stream.Close()
			s.release(clientID)
			return nil, err
		}
		defer stream.SetDeadline(timeZero())
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeRemoteControlStart, start)
	if err != nil {
		_ = stream.Close()
		s.release(clientID)
		return nil, err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		_ = stream.Close()
		s.release(clientID)
		return nil, err
	}

	response, err := protocol.ReadMessage(stream)
	if err != nil {
		_ = stream.Close()
		s.release(clientID)
		return nil, err
	}

	switch response.Type {
	case protocol.MsgTypeRemoteControlReady:
		var ready protocol.RemoteControlReady
		if err := response.ParsePayload(&ready); err != nil {
			_ = stream.Close()
			s.release(clientID)
			return nil, err
		}
		if ready.Width <= 0 || ready.Height <= 0 {
			_ = stream.Close()
			s.release(clientID)
			return nil, fmt.Errorf("remote control ready payload missing desktop size")
		}
		return &RemoteControlSession{
			ClientID: clientID,
			Stream:   stream,
			Ready:    ready,
			release: func() {
				s.release(clientID)
			},
		}, nil
	case protocol.MsgTypeRemoteControlError:
		var remoteErr protocol.RemoteControlError
		if err := response.ParsePayload(&remoteErr); err != nil {
			_ = stream.Close()
			s.release(clientID)
			return nil, err
		}
		_ = stream.Close()
		s.release(clientID)
		return nil, fmt.Errorf("remote control setup failed: %s", remoteErr.Message)
	default:
		_ = stream.Close()
		s.release(clientID)
		return nil, fmt.Errorf("unexpected remote control response type: %d", response.Type)
	}
}

func (s *remoteControlService) tryAcquire(clientID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.active[clientID]; exists {
		return false
	}
	s.active[clientID] = struct{}{}
	return true
}

func (s *remoteControlService) release(clientID string) {
	s.mu.Lock()
	delete(s.active, clientID)
	s.mu.Unlock()
}

var now = func() time.Time { return time.Now() }

func timeZero() time.Time { return time.Time{} }
