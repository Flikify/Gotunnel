//go:build !(windows || (linux && !android))

package utils

import "fmt"

// CaptureScreenshot is not available on this platform.
func CaptureScreenshot(quality int) ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("screenshot not supported on this platform")
}

// CaptureAllScreens is not available on this platform.
func CaptureAllScreens(quality int) ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("screenshot not supported on this platform")
}
