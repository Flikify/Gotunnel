package service

import (
	"fmt"
	"net"
	"time"

	"github.com/gotunnel/pkg/observability"
	"github.com/gotunnel/pkg/protocol"
)

const (
	minSystemStatsTimeout = 10 * time.Second
	minScreenshotTimeout  = 15 * time.Second
)

// RemoteOpsRuntime exposes the runtime hooks required by remote operations.
type RemoteOpsRuntime interface {
	IsClientOnline(clientID string) bool
	OpenClientStream(clientID string) (net.Conn, error)
	ClientResponseTimeout() time.Duration
	LocalDiagnosticStore() *observability.DiagnosticStore
}

// RemoteOpsService coordinates status and screenshot operations.
type RemoteOpsService interface {
	IsClientOnline(clientID string) bool
	GetClientSystemStats(clientID string) (*protocol.SystemStatsResponse, error)
	GetClientScreenshot(clientID string, quality int) (*protocol.ScreenshotResponse, error)
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

func (s *remoteOpsService) remoteOpTimeout(minTimeout time.Duration) time.Duration {
	timeout := s.runtime.ClientResponseTimeout()
	if timeout < minTimeout {
		return minTimeout
	}
	return timeout
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
