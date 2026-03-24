package handler

import (
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/pkg/observability"
)

type ObservabilityHandler struct {
	events      service.EventService
	diagnostics service.DiagnosticsService
}

func NewObservabilityHandler(events service.EventService, diagnostics service.DiagnosticsService) *ObservabilityHandler {
	return &ObservabilityHandler{
		events:      events,
		diagnostics: diagnostics,
	}
}

func (h *ObservabilityHandler) ListEvents(c *gin.Context) {
	filter := observability.EventFilter{
		NodeID:    c.Query("node_id"),
		NodeRole:  c.Query("node_role"),
		Category:  c.Query("category"),
		Severity:  c.Query("severity"),
		EventCode: c.Query("event_code"),
	}
	if limit, err := strconv.Atoi(c.DefaultQuery("limit", "100")); err == nil {
		filter.Limit = limit
	}

	events, err := h.events.ListEvents(filter)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	Success(c, gin.H{"events": events})
}

func (h *ObservabilityHandler) Health(c *gin.Context) {
	limit := 100
	if value, err := strconv.Atoi(c.DefaultQuery("limit", "100")); err == nil {
		limit = value
	}

	health, err := h.events.ListNodeHealth(limit)
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	Success(c, gin.H{"nodes": health})
}

func (h *ObservabilityHandler) QueryDiagnostics(c *gin.Context) {
	nodeID := c.Param("id")
	query := buildDiagnosticsQuery(c)
	query.Follow = false

	page, err := h.diagnostics.QueryNodeDiagnostics(nodeID, query)
	if err != nil {
		BadGateway(c, err.Error())
		return
	}
	Success(c, page)
}

func (h *ObservabilityHandler) StreamDiagnostics(c *gin.Context) {
	nodeID := c.Param("id")
	query := buildDiagnosticsQuery(c)
	query.Follow = true

	records, cancel, err := h.diagnostics.StreamNodeDiagnostics(nodeID, query)
	if err != nil {
		BadGateway(c, err.Error())
		return
	}
	defer cancel()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	clientGone := c.Request.Context().Done()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			return false
		case record, ok := <-records:
			if !ok {
				return false
			}
			c.SSEvent("diagnostic", record)
			return true
		case <-time.After(30 * time.Second):
			c.SSEvent("heartbeat", gin.H{"ts": time.Now().UnixMilli()})
			return true
		}
	})
}

func buildDiagnosticsQuery(c *gin.Context) observability.LogQuery {
	query := observability.LogQuery{
		Level:           c.Query("level"),
		Component:       c.Query("component"),
		EventCodePrefix: c.Query("event_code_prefix"),
		TextContains:    c.Query("text_contains"),
		Cursor:          c.Query("cursor"),
	}
	if value, err := strconv.Atoi(c.DefaultQuery("limit", "100")); err == nil {
		query.Limit = value
	}
	if value, err := strconv.ParseInt(c.Query("time_from"), 10, 64); err == nil {
		query.TimeFrom = value
	}
	if value, err := strconv.ParseInt(c.Query("time_to"), 10, 64); err == nil {
		query.TimeTo = value
	}
	return query
}
