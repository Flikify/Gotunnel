package service

import (
	"fmt"
	"net"
	"time"

	serverruntime "github.com/gotunnel/internal/server/runtime"
	"github.com/gotunnel/pkg/observability"
	"github.com/gotunnel/pkg/protocol"
)

const (
	minSystemStatsTimeout = 10 * time.Second
	minScreenshotTimeout  = 15 * time.Second
	defaultShellTimeout   = 30 * time.Second
	shellTimeoutGrace     = 5 * time.Second
)

// RemoteOpsRuntime exposes the runtime hooks required by remote operations.
type RemoteOpsRuntime interface {
	IsClientOnline(clientID string) bool
	OpenClientStream(clientID string) (net.Conn, error)
	ClientResponseTimeout() time.Duration
	LogSessions() *serverruntime.LogSessionManager
	LocalDiagnosticStore() *observability.DiagnosticStore
}

// RemoteOpsService coordinates log, status, screenshot, and shell operations.
type RemoteOpsService interface {
	IsClientOnline(clientID string) bool
	StartClientLogStream(clientID, sessionID string, lines int, follow bool, level string) (<-chan protocol.LogEntry, error)
	StopClientLogStream(sessionID string)
	GetClientSystemStats(clientID string) (*protocol.SystemStatsResponse, error)
	GetClientScreenshot(clientID string, quality int) (*protocol.ScreenshotResponse, error)
	ExecuteClientShell(clientID, command string, timeout int) (*protocol.ShellExecuteResponse, error)
}

type remoteOpsService struct {
	runtime RemoteOpsRuntime
}

// NewRemoteOpsService creates a remote-ops service backed by the tunnel runtime.
func NewRemoteOpsService(runtime RemoteOpsRuntime) RemoteOpsService {
	return &remoteOpsService{runtime: runtime}
}

func (s *remoteOpsService) IsClientOnline(clientID string) bool {
	return s.runtime.IsClientOnline(clientID)
}

func (s *remoteOpsService) StartClientLogStream(clientID, sessionID string, lines int, follow bool, level string) (<-chan protocol.LogEntry, error) {
	stream, err := s.runtime.OpenClientStream(clientID)
	if err != nil {
		return nil, err
	}

	req := protocol.LogRequest{
		SessionID: sessionID,
		Lines:     lines,
		Follow:    follow,
		Level:     level,
	}
	msg, err := protocol.NewMessage(protocol.MsgTypeLogRequest, req)
	if err != nil {
		stream.Close()
		return nil, err
	}
	if err := protocol.WriteMessage(stream, msg); err != nil {
		stream.Close()
		return nil, err
	}

	session := s.runtime.LogSessions().CreateSession(clientID, sessionID, stream)
	listener := session.AddListener()
	go s.readClientLogs(session)
	return listener, nil
}

func (s *remoteOpsService) StopClientLogStream(sessionID string) {
	session := s.runtime.LogSessions().GetSession(sessionID)
	if session == nil {
		return
	}

	if stream, err := s.runtime.OpenClientStream(session.ClientID); err == nil {
		_ = withStreamDeadline(stream, s.runtime.ClientResponseTimeout(), func() error {
			defer stream.Close()
			req := protocol.LogStopRequest{SessionID: sessionID}
			msg, err := protocol.NewMessage(protocol.MsgTypeLogStop, req)
			if err != nil {
				return err
			}
			return protocol.WriteMessage(stream, msg)
		})
	}

	s.runtime.LogSessions().RemoveSession(sessionID)
}

func (s *remoteOpsService) GetClientSystemStats(clientID string) (*protocol.SystemStatsResponse, error) {
	stream, err := s.runtime.OpenClientStream(clientID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var stats protocol.SystemStatsResponse
	if err := requestResponse(stream, s.remoteOpTimeout(minSystemStatsTimeout), protocol.MsgTypeSystemStatsRequest, nil, protocol.MsgTypeSystemStatsResponse, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (s *remoteOpsService) GetClientScreenshot(clientID string, quality int) (*protocol.ScreenshotResponse, error) {
	stream, err := s.runtime.OpenClientStream(clientID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := protocol.ScreenshotRequest{Quality: quality}
	var screenshot protocol.ScreenshotResponse
	if err := requestResponse(stream, s.remoteOpTimeout(minScreenshotTimeout), protocol.MsgTypeScreenshotRequest, req, protocol.MsgTypeScreenshotResponse, &screenshot); err != nil {
		return nil, err
	}
	if screenshot.Error != "" {
		return nil, fmt.Errorf("screenshot failed: %s", screenshot.Error)
	}
	return &screenshot, nil
}

func (s *remoteOpsService) ExecuteClientShell(clientID, command string, timeout int) (*protocol.ShellExecuteResponse, error) {
	stream, err := s.runtime.OpenClientStream(clientID)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	timeoutSec, streamTimeout := s.shellTimeout(timeout)
	req := protocol.ShellExecuteRequest{
		Command: command,
		Timeout: timeoutSec,
	}

	var result protocol.ShellExecuteResponse
	if err := requestResponse(stream, streamTimeout, protocol.MsgTypeShellExecuteRequest, req, protocol.MsgTypeShellExecuteResponse, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *remoteOpsService) readClientLogs(session *serverruntime.LogSession) {
	defer s.runtime.LogSessions().RemoveSession(session.ID)

	for {
		msg, err := protocol.ReadMessage(session.Stream)
		if err != nil {
			return
		}
		if msg.Type != protocol.MsgTypeLogData {
			continue
		}

		var data protocol.LogData
		if err := msg.ParsePayload(&data); err != nil {
			continue
		}

		for _, entry := range data.Entries {
			session.Broadcast(entry)
		}
		if data.EOF {
			return
		}
	}
}

func (s *remoteOpsService) remoteOpTimeout(minTimeout time.Duration) time.Duration {
	timeout := s.runtime.ClientResponseTimeout()
	if timeout < minTimeout {
		return minTimeout
	}
	return timeout
}

func (s *remoteOpsService) shellTimeout(timeout int) (int, time.Duration) {
	if timeout <= 0 {
		timeout = int(defaultShellTimeout / time.Second)
	}

	streamTimeout := time.Duration(timeout)*time.Second + shellTimeoutGrace
	if base := s.runtime.ClientResponseTimeout(); base > streamTimeout {
		streamTimeout = base
	}
	return timeout, streamTimeout
}

func requestResponse(stream net.Conn, timeout time.Duration, requestType uint8, requestPayload any, responseType uint8, responsePayload any) error {
	return withStreamDeadline(stream, timeout, func() error {
		msg, err := protocol.NewMessage(requestType, requestPayload)
		if err != nil {
			return err
		}
		if err := protocol.WriteMessage(stream, msg); err != nil {
			return err
		}

		resp, err := protocol.ReadMessage(stream)
		if err != nil {
			return err
		}
		if resp.Type != responseType {
			return fmt.Errorf("unexpected response type: %d", resp.Type)
		}
		if responsePayload == nil {
			return nil
		}
		return resp.ParsePayload(responsePayload)
	})
}

func withStreamDeadline(stream net.Conn, timeout time.Duration, fn func() error) error {
	if timeout > 0 {
		if err := stream.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
		defer stream.SetDeadline(time.Time{})
	}
	return fn()
}
