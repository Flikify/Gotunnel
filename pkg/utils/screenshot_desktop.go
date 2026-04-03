//go:build windows || (linux && !android)

package utils

import (
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
)

// CaptureScreenshot captures the primary display.
func CaptureScreenshot(quality int) ([]byte, int, int, error) {
	img, err := CapturePrimaryDisplayImage()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("capture screen: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Empty() {
		return nil, 0, 0, fmt.Errorf("failed to get display bounds")
	}

	return encodeJPEG(img, quality)
}

// CapturePrimaryDisplayImage captures the primary display as an RGBA image.
func CapturePrimaryDisplayImage() (*image.RGBA, error) {
	img, err := capturePrimaryScreenshot()
	if err != nil {
		return nil, annotateScreenshotError(err)
	}
	return img, nil
}

func capturePrimaryScreenshot() (*image.RGBA, error) {
	primaryIndex := PrimaryDisplayIndex()
	img, err := screenshot.CaptureDisplay(primaryIndex)
	if err == nil {
		return img, nil
	}

	lastErr := fmt.Errorf("display %d: %w", primaryIndex, err)
	displayCount := screenshot.NumActiveDisplays()
	for i := 0; i < displayCount; i++ {
		if i == primaryIndex {
			continue
		}

		fallback, fallbackErr := screenshot.CaptureDisplay(i)
		if fallbackErr == nil {
			return fallback, nil
		}
		lastErr = fmt.Errorf("%w; display %d: %v", lastErr, i, fallbackErr)
	}

	return nil, lastErr
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
		return nil, 0, 0, fmt.Errorf("capture screen: %w", annotateScreenshotError(err))
	}

	return encodeJPEG(img, quality)
}
