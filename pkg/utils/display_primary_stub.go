//go:build !windows && (!linux || android)

package utils

import "image"

// PrimaryDisplayIndex returns the active display that should be treated as primary.
func PrimaryDisplayIndex() int {
	return 0
}

// PrimaryDisplayBounds returns the bounds of the detected primary display.
func PrimaryDisplayBounds() image.Rectangle {
	return image.Rectangle{}
}
