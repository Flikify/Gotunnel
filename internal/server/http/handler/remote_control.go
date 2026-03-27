package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/pkg/protocol"
)

var remoteControlUpgrader = websocket.Upgrader{
	CheckOrigin: sameOriginWebSocketRequest,
}

type RemoteControlHandler struct {
	service service.RemoteControlService
}

type remoteControlWSMessage struct {
	Type      string   `json:"type"`
	EventType string   `json:"event_type,omitempty"`
	X         float64  `json:"x,omitempty"`
	Y         float64  `json:"y,omitempty"`
	Button    string   `json:"button,omitempty"`
	DeltaX    int      `json:"delta_x,omitempty"`
	DeltaY    int      `json:"delta_y,omitempty"`
	Key       string   `json:"key,omitempty"`
	Keys      []string `json:"keys,omitempty"`
	Text      string   `json:"text,omitempty"`
	Data      []byte   `json:"data,omitempty"`
	Width     int      `json:"width,omitempty"`
	Height    int      `json:"height,omitempty"`
	Timestamp int64    `json:"timestamp,omitempty"`
	Message   string   `json:"message,omitempty"`
	Reason    string   `json:"reason,omitempty"`

	FrameIntervalMS int `json:"frame_interval_ms,omitempty"`
}

type remoteControlPipeResult struct {
	reason      string
	sendStopped bool
}

func NewRemoteControlHandler(service service.RemoteControlService) *RemoteControlHandler {
	return &RemoteControlHandler{service: service}
}

func (h *RemoteControlHandler) Stream(c *gin.Context) {
	clientID := c.Param("id")

	wsConn, err := remoteControlUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	session, err := h.service.OpenSession(clientID, protocol.RemoteControlStart{
		Quality:         queryInt(c, "quality"),
		MaxSide:         queryInt(c, "max_side"),
		FrameIntervalMS: queryInt(c, "frame_interval_ms"),
	})
	if err != nil {
		_ = wsConn.WriteJSON(remoteControlWSMessage{Type: "error", Message: err.Error()})
		return
	}
	defer session.Stop("remote control session closed")

	var wsWriteMu sync.Mutex
	if err := writeRemoteControlWSMessage(wsConn, &wsWriteMu, remoteControlWSMessage{
		Type:            "ready",
		Width:           session.Ready.Width,
		Height:          session.Ready.Height,
		FrameIntervalMS: session.Ready.FrameIntervalMS,
	}); err != nil {
		return
	}

	results := make(chan remoteControlPipeResult, 2)
	var streamWriteMu sync.Mutex

	go h.forwardClientToBrowser(wsConn, session.Stream, &wsWriteMu, results)
	go h.forwardBrowserToClient(wsConn, session.Stream, &wsWriteMu, &streamWriteMu, results)

	result := <-results
	session.Stop(result.reason)

	if result.sendStopped {
		_ = writeRemoteControlWSMessage(wsConn, &wsWriteMu, remoteControlWSMessage{
			Type:   "stopped",
			Reason: result.reason,
		})
	}
}

func (h *RemoteControlHandler) forwardClientToBrowser(wsConn *websocket.Conn, stream net.Conn, wsWriteMu *sync.Mutex, results chan<- remoteControlPipeResult) {
	for {
		msg, err := protocol.ReadMessage(stream)
		if err != nil {
			results <- remoteControlPipeResult{reason: "client disconnected", sendStopped: true}
			return
		}

		switch msg.Type {
		case protocol.MsgTypeRemoteControlFrame:
			var frame protocol.RemoteControlFrame
			if err := msg.ParsePayload(&frame); err != nil {
				_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{Type: "error", Message: err.Error()})
				results <- remoteControlPipeResult{reason: "invalid remote control frame", sendStopped: true}
				return
			}
			if err := writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{
				Type:      "frame",
				Data:      frame.Data,
				Width:     frame.Width,
				Height:    frame.Height,
				Timestamp: frame.Timestamp,
			}); err != nil {
				results <- remoteControlPipeResult{reason: "browser disconnected", sendStopped: false}
				return
			}
		case protocol.MsgTypeRemoteControlClipboardData:
			var clip protocol.RemoteControlClipboardData
			if err := msg.ParsePayload(&clip); err != nil {
				_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{Type: "error", Message: err.Error()})
				results <- remoteControlPipeResult{reason: "invalid clipboard payload", sendStopped: true}
				return
			}
			if err := writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{
				Type: "clipboard_data",
				Text: clip.Text,
			}); err != nil {
				results <- remoteControlPipeResult{reason: "browser disconnected", sendStopped: false}
				return
			}
		case protocol.MsgTypeRemoteControlError:
			var remoteErr protocol.RemoteControlError
			if err := msg.ParsePayload(&remoteErr); err != nil {
				remoteErr.Message = err.Error()
			}
			_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{Type: "error", Message: remoteErr.Message})
			results <- remoteControlPipeResult{reason: remoteErr.Message, sendStopped: true}
			return
		case protocol.MsgTypeRemoteControlStop:
			var stop protocol.RemoteControlStop
			if err := msg.ParsePayload(&stop); err != nil {
				stop.Reason = "remote control stopped"
			}
			results <- remoteControlPipeResult{reason: stop.Reason, sendStopped: true}
			return
		default:
			_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{
				Type:    "error",
				Message: fmt.Sprintf("unexpected remote control message: %d", msg.Type),
			})
			results <- remoteControlPipeResult{reason: "unexpected client message", sendStopped: true}
			return
		}
	}
}

func (h *RemoteControlHandler) forwardBrowserToClient(wsConn *websocket.Conn, stream net.Conn, wsWriteMu *sync.Mutex, streamWriteMu *sync.Mutex, results chan<- remoteControlPipeResult) {
	for {
		_, payload, err := wsConn.ReadMessage()
		if err != nil {
			results <- remoteControlPipeResult{reason: "browser disconnected", sendStopped: false}
			return
		}

		var msg remoteControlWSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{Type: "error", Message: err.Error()})
			results <- remoteControlPipeResult{reason: "invalid browser message", sendStopped: true}
			return
		}

		switch msg.Type {
		case "input":
			if err := writeProtocolWithLock(stream, streamWriteMu, protocol.MsgTypeRemoteControlInput, protocol.RemoteControlInput{
				EventType: msg.EventType,
				X:         msg.X,
				Y:         msg.Y,
				Button:    msg.Button,
				DeltaX:    msg.DeltaX,
				DeltaY:    msg.DeltaY,
				Key:       msg.Key,
				Keys:      msg.Keys,
				Text:      msg.Text,
			}); err != nil {
				_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{Type: "error", Message: err.Error()})
				results <- remoteControlPipeResult{reason: "client control stream closed", sendStopped: true}
				return
			}
		case "clipboard_get":
			if err := writeProtocolWithLock(stream, streamWriteMu, protocol.MsgTypeRemoteControlClipboardGet, protocol.RemoteControlClipboardGet{}); err != nil {
				_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{Type: "error", Message: err.Error()})
				results <- remoteControlPipeResult{reason: "client control stream closed", sendStopped: true}
				return
			}
		case "clipboard_set":
			if err := writeProtocolWithLock(stream, streamWriteMu, protocol.MsgTypeRemoteControlClipboardSet, protocol.RemoteControlClipboardSet{Text: msg.Text}); err != nil {
				_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{Type: "error", Message: err.Error()})
				results <- remoteControlPipeResult{reason: "client control stream closed", sendStopped: true}
				return
			}
		case "stop":
			results <- remoteControlPipeResult{reason: msg.Reason, sendStopped: true}
			return
		default:
			_ = writeRemoteControlWSMessage(wsConn, wsWriteMu, remoteControlWSMessage{
				Type:    "error",
				Message: fmt.Sprintf("unsupported browser message type: %s", msg.Type),
			})
			results <- remoteControlPipeResult{reason: "unsupported browser message", sendStopped: true}
			return
		}
	}
}

func writeProtocolWithLock(stream net.Conn, mu *sync.Mutex, messageType uint8, payload any) error {
	msg, err := protocol.NewMessage(messageType, payload)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	return protocol.WriteMessage(stream, msg)
}

func writeRemoteControlWSMessage(conn *websocket.Conn, mu *sync.Mutex, payload remoteControlWSMessage) error {
	mu.Lock()
	defer mu.Unlock()
	return conn.WriteJSON(payload)
}

func queryInt(c *gin.Context, key string) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func sameOriginWebSocketRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if originURL.Scheme != "http" && originURL.Scheme != "https" {
		return false
	}

	originHost, originPort := splitHostPort(originURL.Host)
	requestScheme := "http"
	if r.TLS != nil {
		requestScheme = "https"
	}
	requestHost, requestPort := splitHostPort(r.Host)

	return strings.EqualFold(originHost, requestHost) && effectivePort(originPort, originURL.Scheme) == effectivePort(requestPort, requestScheme)
}

func splitHostPort(hostport string) (string, string) {
	host, port, err := net.SplitHostPort(hostport)
	if err == nil {
		return host, port
	}
	return hostport, ""
}

func effectivePort(port, scheme string) string {
	if port != "" {
		return port
	}
	if scheme == "https" {
		return "443"
	}
	return "80"
}
