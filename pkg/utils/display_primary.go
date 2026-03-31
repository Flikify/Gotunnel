package utils

import "image"

func selectPrimaryDisplayIndex(bounds []image.Rectangle) int {
	for i, rect := range bounds {
		if rect.Dx() <= 0 || rect.Dy() <= 0 {
			continue
		}
		if rect.Min.X <= 0 && 0 < rect.Max.X && rect.Min.Y <= 0 && 0 < rect.Max.Y {
			return i
		}
	}
	return 0
}
