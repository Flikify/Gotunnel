package runtime

import (
	"time"

	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/observability"
)

type eventIngestor struct {
	store db.OperationalEventStore
}

func newEventIngestor(store db.OperationalEventStore) *eventIngestor {
	return &eventIngestor{store: store}
}

func (i *eventIngestor) Ingest(events []observability.OperationalEvent) error {
	if i == nil || i.store == nil || len(events) == 0 {
		return nil
	}

	normalized := make([]observability.OperationalEvent, 0, len(events))
	now := time.Now()
	for _, event := range events {
		normalized = append(normalized, event.Normalize(now))
	}
	if err := i.store.AppendOperationalEvents(normalized); err != nil {
		return err
	}

	incidents := make([]observability.OperationalEvent, 0, 2)
	for _, event := range normalized {
		switch event.EventCode {
		case observability.EventClientUpdateFailed:
			incidents = append(incidents, incidentFromEvent(event, observability.EventIncidentUpdateFailed, "Client update failed"))
		case observability.EventServerHeartbeatTimeout:
			incidents = append(incidents, incidentFromEvent(event, observability.EventServerHeartbeatTimeout, "Client heartbeat timeout"))
		case observability.EventServerProxyBindFailed:
			incidents = append(incidents, incidentFromEvent(event, observability.EventIncidentProxyBindFailed, "Server proxy bind failed"))
		}

		switch event.EventCode {
		case observability.EventClientAuthRejected:
			if count, err := i.store.CountOperationalEventsSince(event.NodeID, event.EventCode, now.Add(-10*time.Minute).UnixMilli()); err == nil && count == 3 {
				incidents = append(incidents, incidentFromEvent(event, observability.EventIncidentAuthFailures, "Repeated client auth failures"))
			}
		case observability.EventClientReconnectBackoff:
			if count, err := i.store.CountOperationalEventsSince(event.NodeID, event.EventCode, now.Add(-10*time.Minute).UnixMilli()); err == nil && count == 3 {
				incidents = append(incidents, incidentFromEvent(event, observability.EventIncidentReconnectStorm, "Frequent reconnect backoff detected"))
			}
		case observability.EventAndroidNetworkLost:
			if count, err := i.store.CountOperationalEventsSince(event.NodeID, event.EventCode, now.Add(-10*time.Minute).UnixMilli()); err == nil && count == 3 {
				incidents = append(incidents, incidentFromEvent(event, observability.EventIncidentAndroidFlapping, "Android network flapping detected"))
			}
		}
	}

	if len(incidents) == 0 {
		return nil
	}
	return i.store.AppendOperationalEvents(incidents)
}

func incidentFromEvent(event observability.OperationalEvent, incidentCode, summary string) observability.OperationalEvent {
	return observability.OperationalEvent{
		Timestamp: event.Timestamp,
		Severity:  observability.SeverityCritical,
		NodeID:    event.NodeID,
		NodeRole:  event.NodeRole,
		Category:  observability.CategoryIncident,
		EventCode: incidentCode,
		Summary:   summary,
		Fields:    event.Fields,
		Corr:      event.Corr,
	}
}
