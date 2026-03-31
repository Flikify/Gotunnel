package utils

import (
	"image"
	"testing"
)

func TestSelectPrimaryDisplayIndexPrefersDisplayContainingOrigin(t *testing.T) {
	bounds := []image.Rectangle{
		image.Rect(1920, 0, 3840, 1080),
		image.Rect(0, 0, 1920, 1080),
	}

	got := selectPrimaryDisplayIndex(bounds)
	if got != 1 {
		t.Fatalf("unexpected primary display index: got %d want %d", got, 1)
	}
}

func TestSelectPrimaryDisplayIndexFallsBackToFirstDisplay(t *testing.T) {
	bounds := []image.Rectangle{
		image.Rect(1920, 0, 3840, 1080),
		image.Rect(-1920, 0, 0, 1080),
	}

	got := selectPrimaryDisplayIndex(bounds)
	if got != 0 {
		t.Fatalf("unexpected fallback display index: got %d want %d", got, 0)
	}
}

func TestSelectPrimaryDisplayIndexSkipsEmptyBounds(t *testing.T) {
	bounds := []image.Rectangle{
		image.Rect(0, 0, 0, 1080),
		image.Rect(0, 0, 1920, 1080),
	}

	got := selectPrimaryDisplayIndex(bounds)
	if got != 1 {
		t.Fatalf("unexpected primary display index after skipping empty bounds: got %d want %d", got, 1)
	}
}
