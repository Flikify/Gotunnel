package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
)

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
