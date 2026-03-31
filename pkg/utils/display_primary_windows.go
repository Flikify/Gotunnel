//go:build windows

package utils

import (
	"image"

	"github.com/kbinani/screenshot"
)

// PrimaryDisplayIndex returns the active display that contains the desktop origin.
func PrimaryDisplayIndex() int {
	displayCount := screenshot.NumActiveDisplays()
	if displayCount <= 0 {
		return 0
	}

	bounds := make([]image.Rectangle, 0, displayCount)
	for i := 0; i < displayCount; i++ {
		bounds = append(bounds, screenshot.GetDisplayBounds(i))
	}

	return selectPrimaryDisplayIndex(bounds)
}

// PrimaryDisplayBounds returns the bounds of the detected primary display.
func PrimaryDisplayBounds() image.Rectangle {
	return screenshot.GetDisplayBounds(PrimaryDisplayIndex())
}
