package tunnel

import (
	"fmt"
	"image"
)

type RemoteController interface {
	Capture() (image.Image, error)
	ReadClipboard() (string, error)
	WriteClipboard(text string) error
	PasteText(text string) error
	MovePointer(x, y int) error
	MouseDown(button string) error
	MouseUp(button string) error
	Click(button string, double bool) error
	Scroll(deltaX, deltaY int) error
	KeyDown(key string) error
	KeyUp(key string) error
	Shortcut(keys []string) error
}

func normalizeMouseButton(button string) string {
	switch button {
	case "", "left", "main":
		return "left"
	case "right", "secondary":
		return "right"
	case "middle", "center", "auxiliary":
		return "center"
	default:
		return "left"
	}
}

func normalizeRobotKey(key string) string {
	switch key {
	case "":
		return ""
	case " ":
		return "space"
	case "control":
		return "ctrl"
	case "meta":
		return "cmd"
	case "command":
		return "cmd"
	case "arrowup":
		return "up"
	case "arrowdown":
		return "down"
	case "arrowleft":
		return "left"
	case "arrowright":
		return "right"
	case "delete":
		return "delete"
	case "escape":
		return "esc"
	default:
		return key
	}
}

func normalizeShortcut(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		normalized := normalizeRobotKey(key)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func errUnsupportedRemoteControl() error {
	return fmt.Errorf("remote control not supported on this platform")
}
