//go:build windows || (linux && !android)

package utils

import (
	"fmt"

	"github.com/kbinani/screenshot"
)

// CaptureScreenshot captures the primary display.
func CaptureScreenshot(quality int) ([]byte, int, int, error) {
	img, err := screenshot.CaptureDisplay(0)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("capture screen: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Empty() {
		return nil, 0, 0, fmt.Errorf("failed to get display bounds")
	}

	return encodeJPEG(img, quality)
}

// CaptureAllScreens captures the full virtual desktop across all connected displays.
func CaptureAllScreens(quality int) ([]byte, int, int, error) {
	displayCount := screenshot.NumActiveDisplays()
	if displayCount <= 0 {
		return nil, 0, 0, fmt.Errorf("capture screen: no active displays")
	}

	virtualBounds := screenshot.GetDisplayBounds(0)
	for i := 1; i < displayCount; i++ {
		virtualBounds = virtualBounds.Union(screenshot.GetDisplayBounds(i))
	}

	img, err := screenshot.CaptureRect(virtualBounds)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("capture screen: %w", err)
	}

	return encodeJPEG(img, quality)
}
