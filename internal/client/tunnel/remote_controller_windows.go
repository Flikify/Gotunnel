//go:build windows

package tunnel

import (
	"fmt"
	"image"
	"math"
	"strings"
	"syscall"

	"github.com/atotto/clipboard"
	"github.com/kbinani/screenshot"
)

const (
	mouseeventfLeftDown   = 0x0002
	mouseeventfLeftUp     = 0x0004
	mouseeventfRightDown  = 0x0008
	mouseeventfRightUp    = 0x0010
	mouseeventfMiddleDown = 0x0020
	mouseeventfMiddleUp   = 0x0040
	mouseeventfWheel      = 0x0800
	mouseeventfHWheel     = 0x01000

	keyeventfKeyUp = 0x0002

	smCXScreen = 0
	smCYScreen = 1
)

var (
	windowsUser32        = syscall.NewLazyDLL("user32.dll")
	procKeybdEvent       = windowsUser32.NewProc("keybd_event")
	procMouseEvent       = windowsUser32.NewProc("mouse_event")
	procSetCursorPos     = windowsUser32.NewProc("SetCursorPos")
	procGetSystemMetrics = windowsUser32.NewProc("GetSystemMetrics")
)

type windowsRemoteController struct{}

func newRemoteController() RemoteController {
	return &windowsRemoteController{}
}

func (c *windowsRemoteController) Capture() (image.Image, error) {
	return screenshot.CaptureDisplay(0)
}

func (c *windowsRemoteController) ReadClipboard() (string, error) {
	return clipboard.ReadAll()
}

func (c *windowsRemoteController) WriteClipboard(text string) error {
	return clipboard.WriteAll(text)
}

func (c *windowsRemoteController) PasteText(text string) error {
	if err := clipboard.WriteAll(text); err != nil {
		return err
	}
	return c.Shortcut([]string{"control", "v"})
}

func (c *windowsRemoteController) MovePointer(x, y int) error {
	x, y = scalePrimaryDisplayPoint(x, y)

	ret, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if ret != 0 {
		return nil
	}
	if err != nil && err != syscall.Errno(0) {
		return fmt.Errorf("set cursor position: %w", err)
	}
	return fmt.Errorf("set cursor position failed")
}

func (c *windowsRemoteController) MouseDown(button string) error {
	return sendMouseButtonEvent(normalizeMouseButton(button), true)
}

func (c *windowsRemoteController) MouseUp(button string) error {
	return sendMouseButtonEvent(normalizeMouseButton(button), false)
}

func (c *windowsRemoteController) Click(button string, double bool) error {
	button = normalizeMouseButton(button)
	if err := sendMouseButtonEvent(button, true); err != nil {
		return err
	}
	if err := sendMouseButtonEvent(button, false); err != nil {
		return err
	}
	if !double {
		return nil
	}
	if err := sendMouseButtonEvent(button, true); err != nil {
		return err
	}
	return sendMouseButtonEvent(button, false)
}

func (c *windowsRemoteController) Scroll(deltaX, deltaY int) error {
	if deltaY != 0 {
		sendMouseEvent(mouseeventfWheel, uint32(int32(deltaY*120)))
	}
	if deltaX != 0 {
		sendMouseEvent(mouseeventfHWheel, uint32(int32(deltaX*120)))
	}
	return nil
}

func (c *windowsRemoteController) KeyDown(key string) error {
	return sendKeyEvent(key, false)
}

func (c *windowsRemoteController) KeyUp(key string) error {
	return sendKeyEvent(key, true)
}

func (c *windowsRemoteController) Shortcut(keys []string) error {
	normalized := normalizeShortcut(keys)
	if len(normalized) == 0 {
		return nil
	}
	if len(normalized) == 1 {
		return tapKey(normalized[0])
	}

	for _, key := range normalized[:len(normalized)-1] {
		if err := sendKeyEvent(key, false); err != nil {
			releaseShortcutKeys(normalized[:len(normalized)-1])
			return err
		}
	}

	if err := tapKey(normalized[len(normalized)-1]); err != nil {
		releaseShortcutKeys(normalized[:len(normalized)-1])
		return err
	}

	releaseShortcutKeys(normalized[:len(normalized)-1])
	return nil
}

func releaseShortcutKeys(keys []string) {
	for i := len(keys) - 1; i >= 0; i-- {
		_ = sendKeyEvent(keys[i], true)
	}
}

func sendMouseButtonEvent(button string, down bool) error {
	flag, ok := mouseButtonFlag(button, down)
	if !ok {
		return fmt.Errorf("unsupported mouse button: %s", button)
	}
	sendMouseEvent(flag, 0)
	return nil
}

func mouseButtonFlag(button string, down bool) (uint32, bool) {
	switch button {
	case "left":
		if down {
			return mouseeventfLeftDown, true
		}
		return mouseeventfLeftUp, true
	case "right":
		if down {
			return mouseeventfRightDown, true
		}
		return mouseeventfRightUp, true
	case "center":
		if down {
			return mouseeventfMiddleDown, true
		}
		return mouseeventfMiddleUp, true
	default:
		return 0, false
	}
}

func sendMouseEvent(flags uint32, data uint32) {
	procMouseEvent.Call(uintptr(flags), 0, 0, uintptr(data), 0)
}

func tapKey(key string) error {
	if err := sendKeyEvent(key, false); err != nil {
		return err
	}
	return sendKeyEvent(key, true)
}

func sendKeyEvent(key string, keyUp bool) error {
	vk, ok := windowsVirtualKey(key)
	if !ok {
		return fmt.Errorf("unsupported key: %s", key)
	}

	flags := uintptr(0)
	if keyUp {
		flags = keyeventfKeyUp
	}
	procKeybdEvent.Call(uintptr(vk), 0, flags, 0)
	return nil
}

func windowsVirtualKey(key string) (uint16, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	if len(key) == 1 {
		switch r := key[0]; {
		case r >= 'a' && r <= 'z':
			return uint16(strings.ToUpper(key)[0]), true
		case r >= '0' && r <= '9':
			return uint16(r), true
		}
	}

	vk, ok := windowsKeyMap[key]
	return vk, ok
}

func scalePrimaryDisplayPoint(x, y int) (int, int) {
	bounds := screenshot.GetDisplayBounds(0)
	logicalWidth := getSystemMetric(smCXScreen)
	logicalHeight := getSystemMetric(smCYScreen)

	if bounds.Dx() <= 0 || bounds.Dy() <= 0 || logicalWidth <= 0 || logicalHeight <= 0 {
		return x, y
	}

	scaleX := float64(bounds.Dx()) / float64(logicalWidth)
	scaleY := float64(bounds.Dy()) / float64(logicalHeight)

	if scaleX <= 0 || scaleY <= 0 {
		return x, y
	}

	return int(math.Round(float64(x) / scaleX)), int(math.Round(float64(y) / scaleY))
}

func getSystemMetric(metric int) int {
	ret, _, _ := procGetSystemMetrics.Call(uintptr(metric))
	return int(int32(ret))
}

var windowsKeyMap = map[string]uint16{
	"alt":       0x12,
	"backspace": 0x08,
	"capslock":  0x14,
	"cmd":       0x5B,
	"command":   0x5B,
	"control":   0x11,
	"ctrl":      0x11,
	"delete":    0x2E,
	"down":      0x28,
	"end":       0x23,
	"enter":     0x0D,
	"esc":       0x1B,
	"escape":    0x1B,
	"f1":        0x70,
	"f2":        0x71,
	"f3":        0x72,
	"f4":        0x73,
	"f5":        0x74,
	"f6":        0x75,
	"f7":        0x76,
	"f8":        0x77,
	"f9":        0x78,
	"f10":       0x79,
	"f11":       0x7A,
	"f12":       0x7B,
	"home":      0x24,
	"insert":    0x2D,
	"left":      0x25,
	"pagedown":  0x22,
	"pageup":    0x21,
	"right":     0x27,
	"shift":     0x10,
	"space":     0x20,
	"tab":       0x09,
	"up":        0x26,
	"win":       0x5B,
}
