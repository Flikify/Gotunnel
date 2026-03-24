//go:build !windows && !linux && !darwin

package tunnel

import "image"

type unsupportedRemoteController struct{}

func newRemoteController() RemoteController {
	return &unsupportedRemoteController{}
}

func (c *unsupportedRemoteController) Capture() (image.Image, error) {
	return nil, errUnsupportedRemoteControl()
}
func (c *unsupportedRemoteController) ReadClipboard() (string, error) {
	return "", errUnsupportedRemoteControl()
}
func (c *unsupportedRemoteController) WriteClipboard(string) error {
	return errUnsupportedRemoteControl()
}
func (c *unsupportedRemoteController) PasteText(string) error { return errUnsupportedRemoteControl() }
func (c *unsupportedRemoteController) MovePointer(int, int) error {
	return errUnsupportedRemoteControl()
}
func (c *unsupportedRemoteController) MouseDown(string) error   { return errUnsupportedRemoteControl() }
func (c *unsupportedRemoteController) MouseUp(string) error     { return errUnsupportedRemoteControl() }
func (c *unsupportedRemoteController) Click(string, bool) error { return errUnsupportedRemoteControl() }
func (c *unsupportedRemoteController) Scroll(int, int) error    { return errUnsupportedRemoteControl() }
func (c *unsupportedRemoteController) KeyDown(string) error     { return errUnsupportedRemoteControl() }
func (c *unsupportedRemoteController) KeyUp(string) error       { return errUnsupportedRemoteControl() }
func (c *unsupportedRemoteController) Shortcut([]string) error  { return errUnsupportedRemoteControl() }
