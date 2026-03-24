//go:build windows || linux || darwin

package tunnel

import (
	"image"

	"github.com/go-vgo/robotgo"
)

type robotRemoteController struct{}

func newRemoteController() RemoteController {
	return &robotRemoteController{}
}

func (c *robotRemoteController) Capture() (image.Image, error) {
	return robotgo.CaptureImg()
}

func (c *robotRemoteController) ReadClipboard() (string, error) {
	return robotgo.ReadAll()
}

func (c *robotRemoteController) WriteClipboard(text string) error {
	return robotgo.WriteAll(text)
}

func (c *robotRemoteController) PasteText(text string) error {
	return robotgo.PasteStr(text)
}

func (c *robotRemoteController) MovePointer(x, y int) error {
	robotgo.Move(x, y)
	return nil
}

func (c *robotRemoteController) MouseDown(button string) error {
	return robotgo.Toggle(normalizeMouseButton(button), "down")
}

func (c *robotRemoteController) MouseUp(button string) error {
	return robotgo.Toggle(normalizeMouseButton(button), "up")
}

func (c *robotRemoteController) Click(button string, double bool) error {
	return robotgo.Click(normalizeMouseButton(button), double)
}

func (c *robotRemoteController) Scroll(deltaX, deltaY int) error {
	if deltaY > 0 {
		robotgo.ScrollDir(deltaY, "down")
	} else if deltaY < 0 {
		robotgo.ScrollDir(-deltaY, "up")
	}
	if deltaX > 0 {
		robotgo.ScrollDir(deltaX, "right")
	} else if deltaX < 0 {
		robotgo.ScrollDir(-deltaX, "left")
	}
	return nil
}

func (c *robotRemoteController) KeyDown(key string) error {
	return robotgo.KeyDown(normalizeRobotKey(key))
}

func (c *robotRemoteController) KeyUp(key string) error {
	return robotgo.KeyUp(normalizeRobotKey(key))
}

func (c *robotRemoteController) Shortcut(keys []string) error {
	normalized := normalizeShortcut(keys)
	if len(normalized) == 0 {
		return nil
	}
	if len(normalized) == 1 {
		return robotgo.KeyTap(normalized[0])
	}
	return robotgo.KeyTap(normalized[len(normalized)-1], normalized[:len(normalized)-1])
}
