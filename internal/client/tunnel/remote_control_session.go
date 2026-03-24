package tunnel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"net"
	"sync"
	"time"

	"github.com/gotunnel/pkg/protocol"
	"golang.org/x/image/draw"
)

const (
	remoteControlDefaultQuality         = 55
	remoteControlDefaultMaxSide         = 1440
	remoteControlDefaultFrameIntervalMS = 150
	remoteControlMinimumFrameIntervalMS = 50
)

type remoteControlSession struct {
	controller      RemoteController
	quality         int
	maxSide         int
	frameIntervalMS int

	writeMu sync.Mutex
	stateMu sync.Mutex

	desktopWidth   int
	desktopHeight  int
	pressedKeys    map[string]struct{}
	pressedButtons map[string]struct{}
	lastFrameHash  [32]byte
	hasFrameHash   bool
}

func (c *Client) handleRemoteControlStart(stream net.Conn, msg *protocol.Message) {
	defer stream.Close()

	if !c.features.AllowRemoteControl || c.remoteController == nil {
		writeRemoteControlMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: "remote control not supported on this platform"})
		return
	}

	var req protocol.RemoteControlStart
	if err := msg.ParsePayload(&req); err != nil {
		writeRemoteControlMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
		return
	}

	session := newRemoteControlSession(c.remoteController, req)
	frame, err := session.captureFrame(true)
	if err != nil {
		writeRemoteControlMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
		return
	}

	ready := protocol.RemoteControlReady{
		Width:           frame.Width,
		Height:          frame.Height,
		FrameIntervalMS: session.frameIntervalMS,
	}
	if err := session.writeMessage(stream, protocol.MsgTypeRemoteControlReady, ready); err != nil {
		return
	}
	if err := session.writeMessage(stream, protocol.MsgTypeRemoteControlFrame, frame); err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer session.releaseInputs()

	go session.streamFrames(ctx, stream)

	for {
		incoming, err := protocol.ReadMessage(stream)
		if err != nil {
			return
		}

		switch incoming.Type {
		case protocol.MsgTypeRemoteControlInput:
			var input protocol.RemoteControlInput
			if err := incoming.ParsePayload(&input); err != nil {
				_ = session.writeMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
				continue
			}
			if err := session.handleInput(input); err != nil {
				_ = session.writeMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
			}
		case protocol.MsgTypeRemoteControlClipboardGet:
			text, err := session.controller.ReadClipboard()
			if err != nil {
				_ = session.writeMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
				continue
			}
			_ = session.writeMessage(stream, protocol.MsgTypeRemoteControlClipboardData, protocol.RemoteControlClipboardData{Text: text})
		case protocol.MsgTypeRemoteControlClipboardSet:
			var update protocol.RemoteControlClipboardSet
			if err := incoming.ParsePayload(&update); err != nil {
				_ = session.writeMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
				continue
			}
			if err := session.controller.WriteClipboard(update.Text); err != nil {
				_ = session.writeMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
			}
		case protocol.MsgTypeRemoteControlStop:
			return
		default:
			_ = session.writeMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: fmt.Sprintf("unexpected remote control message: %d", incoming.Type)})
			return
		}
	}
}

func newRemoteControlSession(controller RemoteController, req protocol.RemoteControlStart) *remoteControlSession {
	quality := req.Quality
	if quality <= 0 || quality > 100 {
		quality = remoteControlDefaultQuality
	}

	maxSide := req.MaxSide
	if maxSide <= 0 {
		maxSide = remoteControlDefaultMaxSide
	}

	frameIntervalMS := req.FrameIntervalMS
	if frameIntervalMS <= 0 {
		frameIntervalMS = remoteControlDefaultFrameIntervalMS
	}
	if frameIntervalMS < remoteControlMinimumFrameIntervalMS {
		frameIntervalMS = remoteControlMinimumFrameIntervalMS
	}

	return &remoteControlSession{
		controller:      controller,
		quality:         quality,
		maxSide:         maxSide,
		frameIntervalMS: frameIntervalMS,
		pressedKeys:     make(map[string]struct{}),
		pressedButtons:  make(map[string]struct{}),
	}
}

func (s *remoteControlSession) streamFrames(ctx context.Context, stream net.Conn) {
	ticker := time.NewTicker(time.Duration(s.frameIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			frame, err := s.captureFrame(false)
			if err != nil {
				_ = s.writeMessage(stream, protocol.MsgTypeRemoteControlError, protocol.RemoteControlError{Message: err.Error()})
				_ = stream.Close()
				return
			}
			if frame == nil {
				continue
			}
			if err := s.writeMessage(stream, protocol.MsgTypeRemoteControlFrame, frame); err != nil {
				_ = stream.Close()
				return
			}
		}
	}
}

func (s *remoteControlSession) captureFrame(force bool) (*protocol.RemoteControlFrame, error) {
	img, err := s.controller.Capture()
	if err != nil {
		return nil, fmt.Errorf("capture frame: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("capture frame: empty image")
	}

	s.setDesktopSize(width, height)

	encoded, err := encodeRemoteControlFrame(img, s.maxSide, s.quality)
	if err != nil {
		return nil, err
	}
	if !force && s.isDuplicateFrame(encoded) {
		return nil, nil
	}

	return &protocol.RemoteControlFrame{
		Data:      encoded,
		Width:     width,
		Height:    height,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (s *remoteControlSession) isDuplicateFrame(encoded []byte) bool {
	hash := sha256.Sum256(encoded)

	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.hasFrameHash && s.lastFrameHash == hash {
		return true
	}

	s.lastFrameHash = hash
	s.hasFrameHash = true
	return false
}

func encodeRemoteControlFrame(img image.Image, maxSide, quality int) ([]byte, error) {
	source := img
	if maxSide > 0 {
		source = resizeImage(img, maxSide)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, source, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode remote control frame: %w", err)
	}
	if buf.Len() > protocol.MaxMessageSize {
		return nil, fmt.Errorf("remote control frame exceeds %d bytes", protocol.MaxMessageSize)
	}
	return buf.Bytes(), nil
}

func resizeImage(img image.Image, maxSide int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= maxSide && height <= maxSide {
		return img
	}

	scale := float64(maxSide) / float64(max(width, height))
	targetWidth := max(1, int(math.Round(float64(width)*scale)))
	targetHeight := max(1, int(math.Round(float64(height)*scale)))

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

func (s *remoteControlSession) handleInput(input protocol.RemoteControlInput) error {
	switch input.EventType {
	case "mouse_move":
		x, y := s.absoluteCoordinates(input.X, input.Y)
		return s.controller.MovePointer(x, y)
	case "mouse_down":
		button := normalizeMouseButton(input.Button)
		if err := s.controller.MouseDown(button); err != nil {
			return err
		}
		s.trackButton(button, true)
		return nil
	case "mouse_up":
		button := normalizeMouseButton(input.Button)
		if err := s.controller.MouseUp(button); err != nil {
			return err
		}
		s.trackButton(button, false)
		return nil
	case "mouse_click":
		return s.controller.Click(normalizeMouseButton(input.Button), false)
	case "mouse_double_click":
		return s.controller.Click(normalizeMouseButton(input.Button), true)
	case "mouse_wheel":
		return s.controller.Scroll(input.DeltaX, input.DeltaY)
	case "key_down":
		key := normalizeRobotKey(input.Key)
		if key == "" {
			return nil
		}
		if err := s.controller.KeyDown(key); err != nil {
			return err
		}
		s.trackKey(key, true)
		return nil
	case "key_up":
		key := normalizeRobotKey(input.Key)
		if key == "" {
			return nil
		}
		if err := s.controller.KeyUp(key); err != nil {
			return err
		}
		s.trackKey(key, false)
		return nil
	case "shortcut":
		return s.controller.Shortcut(input.Keys)
	case "paste_text":
		return s.controller.PasteText(input.Text)
	default:
		return fmt.Errorf("unsupported remote control input event: %s", input.EventType)
	}
}

func (s *remoteControlSession) writeMessage(stream net.Conn, messageType uint8, payload any) error {
	msg, err := protocol.NewMessage(messageType, payload)
	if err != nil {
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return protocol.WriteMessage(stream, msg)
}

func writeRemoteControlMessage(stream net.Conn, messageType uint8, payload any) error {
	msg, err := protocol.NewMessage(messageType, payload)
	if err != nil {
		return err
	}
	return protocol.WriteMessage(stream, msg)
}

func (s *remoteControlSession) setDesktopSize(width, height int) {
	s.stateMu.Lock()
	s.desktopWidth = width
	s.desktopHeight = height
	s.stateMu.Unlock()
}

func (s *remoteControlSession) absoluteCoordinates(normX, normY float64) (int, int) {
	s.stateMu.Lock()
	width := s.desktopWidth
	height := s.desktopHeight
	s.stateMu.Unlock()

	if width <= 1 || height <= 1 {
		return 0, 0
	}

	x := clamp(normX, 0, 1)
	y := clamp(normY, 0, 1)

	return int(math.Round(x * float64(width-1))), int(math.Round(y * float64(height-1)))
}

func (s *remoteControlSession) trackKey(key string, pressed bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if pressed {
		s.pressedKeys[key] = struct{}{}
		return
	}
	delete(s.pressedKeys, key)
}

func (s *remoteControlSession) trackButton(button string, pressed bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if pressed {
		s.pressedButtons[button] = struct{}{}
		return
	}
	delete(s.pressedButtons, button)
}

func (s *remoteControlSession) releaseInputs() {
	s.stateMu.Lock()
	keys := make([]string, 0, len(s.pressedKeys))
	for key := range s.pressedKeys {
		keys = append(keys, key)
	}
	buttons := make([]string, 0, len(s.pressedButtons))
	for button := range s.pressedButtons {
		buttons = append(buttons, button)
	}
	s.pressedKeys = make(map[string]struct{})
	s.pressedButtons = make(map[string]struct{})
	s.stateMu.Unlock()

	for _, button := range buttons {
		_ = s.controller.MouseUp(button)
	}
	for _, key := range keys {
		_ = s.controller.KeyUp(key)
	}
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
