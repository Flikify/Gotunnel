package runtime

import (
	"io"
	"log"
	"time"

	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/observability"
	"github.com/gotunnel/pkg/protocol"
)

func (s *Server) SetOperationalEventStore(store db.OperationalEventStore) {
	s.eventStore = store
	s.ingestor = newEventIngestor(store)
}

func (s *Server) SetDiagnosticStore(store *observability.DiagnosticStore) {
	s.diagStore = store
}

func (s *Server) LocalDiagnosticStore() *observability.DiagnosticStore {
	return s.diagStore
}

func (s *Server) IngestOperationalEvents(events []observability.OperationalEvent) error {
	if s.ingestor == nil {
		return nil
	}
	return s.ingestor.Ingest(events)
}

func (s *Server) emitServerEvent(severity, category, eventCode, summary string, fields map[string]string, corr observability.CorrelationContext) {
	_ = s.IngestOperationalEvents([]observability.OperationalEvent{{
		Timestamp: time.Now().UnixMilli(),
		Severity:  severity,
		NodeID:    "server",
		NodeRole:  observability.NodeRoleServer,
		Category:  category,
		EventCode: eventCode,
		Summary:   summary,
		Fields:    fields,
		Corr:      corr,
	}})
}

func (s *Server) handleClientInitiatedStreams(cs *ClientSession) {
	for {
		stream, err := cs.Session.Accept()
		if err != nil {
			return
		}
		go s.handleClientInitiatedStream(cs, stream)
	}
}

func (s *Server) handleClientInitiatedStream(cs *ClientSession, stream io.ReadWriteCloser) {
	defer stream.Close()

	msg, err := protocol.ReadMessage(stream)
	if err != nil {
		return
	}

	switch msg.Type {
	case protocol.MsgTypeOperationalEvents:
		var batch protocol.OperationalEventBatch
		if err := msg.ParsePayload(&batch); err != nil {
			return
		}
		for i := range batch.Events {
			if batch.Events[i].NodeID == "" {
				batch.Events[i].NodeID = cs.ID
			}
			if batch.Events[i].NodeRole == "" {
				batch.Events[i].NodeRole = observability.NodeRoleClient
			}
		}
		if err := s.IngestOperationalEvents(batch.Events); err != nil {
			log.Printf("[Server] ingest operational events failed: %v", err)
		}
	}
}
