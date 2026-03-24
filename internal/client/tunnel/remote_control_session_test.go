package tunnel

import (
	"image"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gotunnel/pkg/protocol"
)

type fakeRemoteController struct {
	mu sync.Mutex

	captureImage     image.Image
	clipboardText    string
	pastedTexts      []string
	writtenClipboard []string
	moves            [][2]int
	mouseDowns       []string
	mouseUps         []string
	clicks           []string
	keyDowns         []string
	keyUps           []string
	shortcuts        [][]string
	scrolls          [][2]int
}

func (f *fakeRemoteController) Capture() (image.Image, error)  { return f.captureImage, nil }
func (f *fakeRemoteController) ReadClipboard() (string, error) { return f.clipboardText, nil }
func (f *fakeRemoteController) WriteClipboard(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writtenClipboard = append(f.writtenClipboard, text)
	f.clipboardText = text
	return nil
}
func (f *fakeRemoteController) PasteText(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pastedTexts = append(f.pastedTexts, text)
	return nil
}
func (f *fakeRemoteController) MovePointer(x, y int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.moves = append(f.moves, [2]int{x, y})
	return nil
}
func (f *fakeRemoteController) MouseDown(button string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mouseDowns = append(f.mouseDowns, button)
	return nil
}
func (f *fakeRemoteController) MouseUp(button string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mouseUps = append(f.mouseUps, button)
	return nil
}
func (f *fakeRemoteController) Click(button string, double bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if double {
		f.clicks = append(f.clicks, "double:"+button)
	} else {
		f.clicks = append(f.clicks, "single:"+button)
	}
	return nil
}
func (f *fakeRemoteController) Scroll(deltaX, deltaY int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scrolls = append(f.scrolls, [2]int{deltaX, deltaY})
	return nil
}
func (f *fakeRemoteController) KeyDown(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keyDowns = append(f.keyDowns, key)
	return nil
}
func (f *fakeRemoteController) KeyUp(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keyUps = append(f.keyUps, key)
	return nil
}
func (f *fakeRemoteController) Shortcut(keys []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	dup := make([]string, len(keys))
	copy(dup, keys)
	f.shortcuts = append(f.shortcuts, dup)
	return nil
}

func TestHandleRemoteControlStartProcessesInputAndClipboard(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	controller := &fakeRemoteController{
		captureImage:  image.NewRGBA(image.Rect(0, 0, 1920, 1080)),
		clipboardText: "remote clipboard",
	}
	features := DefaultPlatformFeatures()
	features.AllowRemoteControl = true
	client := &Client{
		features:         features,
		remoteController: controller,
	}

	done := make(chan struct{})
	go func() {
		client.handleStream(clientConn)
		close(done)
	}()

	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlStart, protocol.RemoteControlStart{
		FrameIntervalMS: 1000,
	})

	ready := readProtocolPayload[protocol.RemoteControlReady](t, serverConn, protocol.MsgTypeRemoteControlReady)
	if ready.Width != 1920 || ready.Height != 1080 {
		t.Fatalf("unexpected ready payload: %+v", ready)
	}

	_ = readProtocolPayload[protocol.RemoteControlFrame](t, serverConn, protocol.MsgTypeRemoteControlFrame)

	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlInput, protocol.RemoteControlInput{EventType: "mouse_move", X: 0.5, Y: 0.25})
	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlInput, protocol.RemoteControlInput{EventType: "shortcut", Keys: []string{"control", "c"}})
	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlInput, protocol.RemoteControlInput{EventType: "paste_text", Text: "paste me"})
	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlClipboardSet, protocol.RemoteControlClipboardSet{Text: "set clipboard"})
	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlClipboardGet, protocol.RemoteControlClipboardGet{})

	clipboard := readUntilMessageType[protocol.RemoteControlClipboardData](t, serverConn, protocol.MsgTypeRemoteControlClipboardData)
	if clipboard.Text != "set clipboard" {
		t.Fatalf("unexpected clipboard text: got %q", clipboard.Text)
	}

	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlStop, protocol.RemoteControlStop{Reason: "done"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("remote control handler did not stop")
	}

	controller.mu.Lock()
	defer controller.mu.Unlock()

	if len(controller.moves) == 0 || controller.moves[0][0] != 960 || controller.moves[0][1] != 270 {
		t.Fatalf("unexpected pointer moves: %+v", controller.moves)
	}
	if len(controller.shortcuts) != 1 || len(controller.shortcuts[0]) != 2 {
		t.Fatalf("unexpected shortcuts: %+v", controller.shortcuts)
	}
	if len(controller.pastedTexts) != 1 || controller.pastedTexts[0] != "paste me" {
		t.Fatalf("unexpected pasted texts: %+v", controller.pastedTexts)
	}
	if len(controller.writtenClipboard) == 0 || controller.writtenClipboard[0] != "set clipboard" {
		t.Fatalf("unexpected clipboard writes: %+v", controller.writtenClipboard)
	}
}

func TestHandleRemoteControlStartReleasesPressedKeysOnDisconnect(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	controller := &fakeRemoteController{
		captureImage: image.NewRGBA(image.Rect(0, 0, 800, 600)),
	}
	features := DefaultPlatformFeatures()
	features.AllowRemoteControl = true
	client := &Client{
		features:         features,
		remoteController: controller,
	}

	done := make(chan struct{})
	go func() {
		client.handleStream(clientConn)
		close(done)
	}()

	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlStart, protocol.RemoteControlStart{
		FrameIntervalMS: 1000,
	})
	_ = readProtocolPayload[protocol.RemoteControlReady](t, serverConn, protocol.MsgTypeRemoteControlReady)
	_ = readProtocolPayload[protocol.RemoteControlFrame](t, serverConn, protocol.MsgTypeRemoteControlFrame)

	writeProtocolMessage(t, serverConn, protocol.MsgTypeRemoteControlInput, protocol.RemoteControlInput{EventType: "key_down", Key: "shift"})
	_ = serverConn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("remote control handler did not exit after disconnect")
	}

	controller.mu.Lock()
	defer controller.mu.Unlock()

	if len(controller.keyDowns) == 0 || controller.keyDowns[0] != "shift" {
		t.Fatalf("unexpected key downs: %+v", controller.keyDowns)
	}
	if len(controller.keyUps) == 0 || controller.keyUps[len(controller.keyUps)-1] != "shift" {
		t.Fatalf("expected shift key to be released on cleanup, got %+v", controller.keyUps)
	}
}

func writeProtocolMessage(t *testing.T, conn net.Conn, messageType uint8, payload any) {
	t.Helper()

	msg, err := protocol.NewMessage(messageType, payload)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if err := protocol.WriteMessage(conn, msg); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}
}

func readProtocolPayload[T any](t *testing.T, conn net.Conn, wantType uint8) T {
	t.Helper()

	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage returned error: %v", err)
	}
	if msg.Type != wantType {
		t.Fatalf("unexpected message type: got %d want %d", msg.Type, wantType)
	}

	var payload T
	if err := msg.ParsePayload(&payload); err != nil {
		t.Fatalf("ParsePayload returned error: %v", err)
	}
	return payload
}

func readUntilMessageType[T any](t *testing.T, conn net.Conn, wantType uint8) T {
	t.Helper()

	for {
		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			t.Fatalf("ReadMessage returned error: %v", err)
		}
		if msg.Type != wantType {
			continue
		}

		var payload T
		if err := msg.ParsePayload(&payload); err != nil {
			t.Fatalf("ParsePayload returned error: %v", err)
		}
		return payload
	}
}
