//go:build windows || linux || darwin

package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/kbinani/screenshot"
)

// CaptureScreenshot captures the primary display.
func CaptureScreenshot(quality int) ([]byte, int, int, error) {
	if quality <= 0 || quality > 100 {
		quality = 75
	}

	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, 0, 0, fmt.Errorf("no active display found")
	}

	bounds := screenshot.GetDisplayBounds(0)
	if bounds.Empty() {
		return nil, 0, 0, fmt.Errorf("failed to get display bounds")
	}

	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("capture screen: %w", err)
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// CaptureAllScreens captures all active displays and stitches them together.
func CaptureAllScreens(quality int) ([]byte, int, int, error) {
	if quality <= 0 || quality > 100 {
		quality = 75
	}

	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, 0, 0, fmt.Errorf("no active display found")
	}

	var totalBounds image.Rectangle
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		totalBounds = totalBounds.Union(bounds)
	}

	totalImg := image.NewRGBA(totalBounds)
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			continue
		}
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				totalImg.Set(x, y, img.At(x-bounds.Min.X, y-bounds.Min.Y))
			}
		}
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, totalImg, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), totalBounds.Dx(), totalBounds.Dy(), nil
}
