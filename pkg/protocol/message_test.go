package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestRemoteControlFrameRoundTrip(t *testing.T) {
	original := RemoteControlFrame{
		Data:      []byte("frame-data"),
		Width:     1920,
		Height:    1080,
		Timestamp: 123456789,
	}

	msg, err := NewMessage(MsgTypeRemoteControlFrame, original)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, msg); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}

	decodedMsg, err := ReadMessage(&buf)
	if err != nil {
		t.Fatalf("ReadMessage returned error: %v", err)
	}
	if decodedMsg.Type != MsgTypeRemoteControlFrame {
		t.Fatalf("unexpected message type: got %d want %d", decodedMsg.Type, MsgTypeRemoteControlFrame)
	}

	var decoded RemoteControlFrame
	if err := decodedMsg.ParsePayload(&decoded); err != nil {
		t.Fatalf("ParsePayload returned error: %v", err)
	}

	if !bytes.Equal(decoded.Data, original.Data) {
		t.Fatalf("unexpected frame data: got %q want %q", decoded.Data, original.Data)
	}
	if decoded.Width != original.Width || decoded.Height != original.Height || decoded.Timestamp != original.Timestamp {
		t.Fatalf("unexpected decoded frame: %+v", decoded)
	}
}

func TestReadMessageRejectsOversizedPayload(t *testing.T) {
	header := make([]byte, HeaderSize)
	header[0] = MsgTypeRemoteControlFrame
	binary.BigEndian.PutUint32(header[1:], uint32(MaxMessageSize+1))

	if _, err := ReadMessage(bytes.NewReader(header)); err == nil {
		t.Fatal("expected oversized payload error")
	}
}
