package sqlite

import (
	"encoding/json"

	"github.com/gotunnel/pkg/observability"
)

func (s *SQLiteStore) AppendOperationalEvents(events []observability.OperationalEvent) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO operational_events (ts, severity, node_id, node_role, category, event_code, summary, fields, corr)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, event := range events {
		fields, err := json.Marshal(event.Fields)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		corr, err := json.Marshal(event.Corr)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := stmt.Exec(
			event.Timestamp,
			event.Severity,
			event.NodeID,
			event.NodeRole,
			event.Category,
			event.EventCode,
			event.Summary,
			string(fields),
			string(corr),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) ListOperationalEvents(filter observability.EventFilter) ([]observability.OperationalEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT ts, severity, node_id, node_role, category, event_code, summary, fields, corr
		FROM operational_events
		WHERE (? = '' OR node_id = ?)
			AND (? = '' OR node_role = ?)
			AND (? = '' OR category = ?)
			AND (? = '' OR severity = ?)
			AND (? = '' OR event_code = ?)
		ORDER BY ts DESC
		LIMIT ?
	`
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		query,
		filter.NodeID, filter.NodeID,
		filter.NodeRole, filter.NodeRole,
		filter.Category, filter.Category,
		filter.Severity, filter.Severity,
		filter.EventCode, filter.EventCode,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []observability.OperationalEvent
	for rows.Next() {
		var event observability.OperationalEvent
		var fieldsJSON, corrJSON string
		if err := rows.Scan(
			&event.Timestamp,
			&event.Severity,
			&event.NodeID,
			&event.NodeRole,
			&event.Category,
			&event.EventCode,
			&event.Summary,
			&fieldsJSON,
			&corrJSON,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(fieldsJSON), &event.Fields)
		_ = json.Unmarshal([]byte(corrJSON), &event.Corr)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) ListNodeHealth(limit int) ([]observability.NodeHealth, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT e.node_id, e.node_role, e.ts, e.severity, e.event_code, e.fields
		FROM operational_events e
		INNER JOIN (
			SELECT node_id, MAX(ts) AS max_ts
			FROM operational_events
			GROUP BY node_id
		) latest ON latest.node_id = e.node_id AND latest.max_ts = e.ts
		ORDER BY e.ts DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var health []observability.NodeHealth
	for rows.Next() {
		var item observability.NodeHealth
		var fieldsJSON string
		if err := rows.Scan(
			&item.NodeID,
			&item.NodeRole,
			&item.LastEventAt,
			&item.LastSeverity,
			&item.LastEventCode,
			&fieldsJSON,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(fieldsJSON), &item.Fields)
		item.IncidentCounts = map[string]int{}
		countRows, err := s.db.Query(`
			SELECT event_code, COUNT(*)
			FROM operational_events
			WHERE node_id = ? AND category = ?
			GROUP BY event_code
		`, item.NodeID, observability.CategoryIncident)
		if err == nil {
			for countRows.Next() {
				var code string
				var count int
				if err := countRows.Scan(&code, &count); err == nil {
					item.IncidentCounts[code] = count
				}
			}
			countRows.Close()
		}
		health = append(health, item)
	}
	return health, rows.Err()
}

func (s *SQLiteStore) CountOperationalEventsSince(nodeID, eventCode string, since int64) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM operational_events
		WHERE (? = '' OR node_id = ?)
			AND (? = '' OR event_code = ?)
			AND ts >= ?
	`, nodeID, nodeID, eventCode, eventCode, since).Scan(&count)
	return count, err
}
