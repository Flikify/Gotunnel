package observability

import "time"

const (
	NodeRoleServer      = "server"
	NodeRoleClient      = "client-runtime"
	NodeRoleAndroidHost = "android-host"
)

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

const (
	CategoryLifecycle = "lifecycle"
	CategoryHealth    = "health"
	CategoryAudit     = "audit"
	CategoryConfig    = "config"
	CategoryUpdate    = "update"
	CategoryIncident  = "incident"
	CategoryNetwork   = "network"
	CategorySecurity  = "security"
)

type CorrelationContext struct {
	RequestID   string `json:"request_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	ClientID    string `json:"client_id,omitempty"`
	StreamID    string `json:"stream_id,omitempty"`
	ProxyRuleID string `json:"proxy_rule_id,omitempty"`
}

type DiagnosticRecord struct {
	Timestamp int64              `json:"ts"`
	Level     string             `json:"level"`
	NodeRole  string             `json:"node_role"`
	NodeID    string             `json:"node_id"`
	Component string             `json:"component"`
	EventCode string             `json:"event_code"`
	Message   string             `json:"message"`
	Fields    map[string]string  `json:"fields,omitempty"`
	Corr      CorrelationContext `json:"corr,omitempty"`
}

type OperationalEvent struct {
	Timestamp int64              `json:"ts"`
	Severity  string             `json:"severity"`
	NodeID    string             `json:"node_id"`
	NodeRole  string             `json:"node_role"`
	Category  string             `json:"category"`
	EventCode string             `json:"event_code"`
	Summary   string             `json:"summary"`
	Fields    map[string]string  `json:"fields,omitempty"`
	Corr      CorrelationContext `json:"corr,omitempty"`
}

type LogQuery struct {
	TimeFrom        int64  `json:"time_from,omitempty"`
	TimeTo          int64  `json:"time_to,omitempty"`
	Level           string `json:"level,omitempty"`
	Component       string `json:"component,omitempty"`
	EventCodePrefix string `json:"event_code_prefix,omitempty"`
	TextContains    string `json:"text_contains,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	Follow          bool   `json:"follow,omitempty"`
}

type LogPage struct {
	Records    []DiagnosticRecord `json:"records"`
	NextCursor string             `json:"next_cursor,omitempty"`
	EOF        bool               `json:"eof"`
}

type EventFilter struct {
	NodeID    string `json:"node_id,omitempty"`
	NodeRole  string `json:"node_role,omitempty"`
	Category  string `json:"category,omitempty"`
	Severity  string `json:"severity,omitempty"`
	EventCode string `json:"event_code,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type NodeHealth struct {
	NodeID         string            `json:"node_id"`
	NodeRole       string            `json:"node_role"`
	LastEventAt    int64             `json:"last_event_at"`
	LastSeverity   string            `json:"last_severity"`
	LastEventCode  string            `json:"last_event_code"`
	IncidentCounts map[string]int    `json:"incident_counts,omitempty"`
	Fields         map[string]string `json:"fields,omitempty"`
}

func (r DiagnosticRecord) Normalize(now time.Time) DiagnosticRecord {
	if r.Timestamp == 0 {
		r.Timestamp = now.UnixMilli()
	}
	if r.Level == "" {
		r.Level = LevelInfo
	}
	if r.EventCode == "" {
		r.EventCode = "legacy.log"
	}
	if r.Fields == nil {
		r.Fields = map[string]string{}
	}
	return r
}

func (e OperationalEvent) Normalize(now time.Time) OperationalEvent {
	if e.Timestamp == 0 {
		e.Timestamp = now.UnixMilli()
	}
	if e.Severity == "" {
		e.Severity = SeverityInfo
	}
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	return e
}
