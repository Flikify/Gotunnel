package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/kbinani/screenshot"
)

// CaptureScreenshot 捕获主屏幕截图
// quality: JPEG 质量 (1-100), 0 使用默认值 (75)
// 返回: JPEG 图片数据, 宽度, 高度, 错误
func CaptureScreenshot(quality int) ([]byte, int, int, error) {
	// 默认质量
	if quality <= 0 || quality > 100 {
		quality = 75
	}

	// 获取活动显示器数量
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, 0, 0, fmt.Errorf("no active display found")
	}

	// 获取主显示器边界
	bounds := screenshot.GetDisplayBounds(0)
	if bounds.Empty() {
		return nil, 0, 0, fmt.Errorf("failed to get display bounds")
	}

	// 捕获屏幕
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("capture screen: %w", err)
	}

	// 编码为 JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// CaptureAllScreens 捕获所有屏幕并拼接
// quality: JPEG 质量 (1-100), 0 使用默认值 (75)
// 返回: JPEG 图片数据, 宽度, 高度, 错误
func CaptureAllScreens(quality int) ([]byte, int, int, error) {
	// 默认质量
	if quality <= 0 || quality > 100 {
		quality = 75
	}

	// 获取活动显示器数量
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, 0, 0, fmt.Errorf("no active display found")
	}

	// 计算所有屏幕的总边界
	var totalBounds image.Rectangle
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		totalBounds = totalBounds.Union(bounds)
	}

	// 创建总画布
	totalImg := image.NewRGBA(totalBounds)

	// 捕获每个屏幕并绘制到总画布
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			continue // 跳过失败的屏幕
		}

		// 绘制到总画布
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				totalImg.Set(x, y, img.At(x-bounds.Min.X, y-bounds.Min.Y))
			}
		}
	}

	// 编码为 JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, totalImg, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), totalBounds.Dx(), totalBounds.Dy(), nil
}
