package service

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/observability"
	"github.com/gotunnel/pkg/protocol"
)

const serverNodeID = "server"

type DiagnosticsService interface {
	QueryNodeDiagnostics(nodeID string, query observability.LogQuery) (observability.LogPage, error)
	StreamNodeDiagnostics(nodeID string, query observability.LogQuery) (<-chan observability.DiagnosticRecord, func(), error)
}

type EventService interface {
	ListEvents(filter observability.EventFilter) ([]observability.OperationalEvent, error)
	ListNodeHealth(limit int) ([]observability.NodeHealth, error)
}

type diagnosticsService struct {
	runtime RemoteOpsRuntime
}

type eventService struct {
	store db.OperationalEventStore
}

func NewDiagnosticsService(runtime RemoteOpsRuntime) DiagnosticsService {
	return &diagnosticsService{runtime: runtime}
}

func NewEventService(store db.OperationalEventStore) EventService {
	return &eventService{store: store}
}

func (s *diagnosticsService) QueryNodeDiagnostics(nodeID string, query observability.LogQuery) (observability.LogPage, error) {
	if nodeID == serverNodeID {
		store := s.runtime.LocalDiagnosticStore()
		if store == nil {
			return observability.LogPage{}, fmt.Errorf("server diagnostic store unavailable")
		}
		return store.Query(query)
	}
	if !s.runtime.IsClientOnline(nodeID) {
		return observability.LogPage{}, fmt.Errorf("client not online")
	}

	stream, err := s.runtime.OpenClientStream(nodeID)
	if err != nil {
		return observability.LogPage{}, err
	}
	defer stream.Close()

	req := protocol.DiagnosticsQueryRequest{
		SessionID: uuid.NewString(),
		Query:     query,
	}
	if err := requestDiagnostics(stream, s.runtime.ClientResponseTimeout(), req); err != nil {
		return observability.LogPage{}, err
	}

	msg, err := protocol.ReadMessage(stream)
	if err != nil {
		return observability.LogPage{}, err
	}
	if msg.Type != protocol.MsgTypeDiagnosticsChunk {
		return observability.LogPage{}, fmt.Errorf("unexpected response type: %d", msg.Type)
	}
	var chunk protocol.DiagnosticsQueryChunk
	if err := msg.ParsePayload(&chunk); err != nil {
		return observability.LogPage{}, err
	}
	return observability.LogPage{
		Records:    chunk.Records,
		NextCursor: chunk.NextCursor,
		EOF:        chunk.EOF,
	}, nil
}

func (s *diagnosticsService) StreamNodeDiagnostics(nodeID string, query observability.LogQuery) (<-chan observability.DiagnosticRecord, func(), error) {
	if nodeID == serverNodeID {
		store := s.runtime.LocalDiagnosticStore()
		if store == nil {
			return nil, nil, fmt.Errorf("server diagnostic store unavailable")
		}

		out := make(chan observability.DiagnosticRecord, 128)
		page, err := store.Query(query)
		if err != nil {
			return nil, nil, err
		}
		var followCh <-chan observability.DiagnosticRecord
		cancelFollow := func() {}
		if query.Follow {
			followCh, cancelFollow, err = store.Follow(query)
			if err != nil {
				return nil, nil, err
			}
		}
		go func() {
			defer cancelFollow()
			defer close(out)
			for _, record := range page.Records {
				out <- record
			}
			if !query.Follow {
				return
			}
			for record := range followCh {
				out <- record
			}
		}()
		return out, cancelFollow, nil
	}

	if !s.runtime.IsClientOnline(nodeID) {
		return nil, nil, fmt.Errorf("client not online")
	}

	stream, err := s.runtime.OpenClientStream(nodeID)
	if err != nil {
		return nil, nil, err
	}

	req := protocol.DiagnosticsQueryRequest{
		SessionID: uuid.NewString(),
		Query:     query,
	}
	if err := requestDiagnostics(stream, s.runtime.ClientResponseTimeout(), req); err != nil {
		stream.Close()
		return nil, nil, err
	}

	out := make(chan observability.DiagnosticRecord, 128)
	done := make(chan struct{})
	go func() {
		defer close(out)
		defer close(done)
		defer stream.Close()
		for {
			msg, err := protocol.ReadMessage(stream)
			if err != nil {
				return
			}
			if msg.Type != protocol.MsgTypeDiagnosticsChunk {
				return
			}
			var chunk protocol.DiagnosticsQueryChunk
			if err := msg.ParsePayload(&chunk); err != nil {
				return
			}
			for _, record := range chunk.Records {
				out <- record
			}
			if chunk.EOF && !query.Follow {
				return
			}
		}
	}()

	cancel := func() {
		_ = stream.Close()
		<-done
	}
	return out, cancel, nil
}

func requestDiagnostics(stream net.Conn, timeout time.Duration, req protocol.DiagnosticsQueryRequest) error {
	return withStreamDeadline(stream, timeout, func() error {
		msg, err := protocol.NewMessage(protocol.MsgTypeDiagnosticsQuery, req)
		if err != nil {
			return err
		}
		return protocol.WriteMessage(stream, msg)
	})
}

func (s *eventService) ListEvents(filter observability.EventFilter) ([]observability.OperationalEvent, error) {
	return s.store.ListOperationalEvents(filter)
}

func (s *eventService) ListNodeHealth(limit int) ([]observability.NodeHealth, error) {
	return s.store.ListNodeHealth(limit)
}
