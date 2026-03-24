//go:build cgo && (windows || (linux && !android) || (darwin && !ios))

package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/go-vgo/robotgo"
)

// CaptureScreenshot captures the primary display.
func CaptureScreenshot(quality int) ([]byte, int, int, error) {
	img, err := robotgo.CaptureImg()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("capture screen: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Empty() {
		return nil, 0, 0, fmt.Errorf("failed to get display bounds")
	}

	return encodeJPEG(img, quality)
}

// CaptureAllScreens currently uses RobotGo's desktop capture path.
func CaptureAllScreens(quality int) ([]byte, int, int, error) {
	img, err := robotgo.CaptureImg()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("capture screen: %w", err)
	}

	return encodeJPEG(img, quality)
}

func encodeJPEG(img image.Image, quality int) ([]byte, int, int, error) {
	var buf bytes.Buffer
	if quality <= 0 || quality > 100 {
		quality = 75
	}

	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}

	bounds := img.Bounds()
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}
