package internal

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/kbinani/screenshot"
)

// Capture 捕获指定显示器的完整屏幕画面。
// display 从 0 开始编号。
func Capture(display int) (*image.RGBA, error) {
	count := screenshot.NumActiveDisplays()

	if display < 0 || display >= count {
		return nil, fmt.Errorf("invalid display index %d, available displays: %d", display, count)
	}

	bounds := screenshot.GetDisplayBounds(display)

	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("capture display %d: %w", display, err)
	}

	return img, nil
}

// EncodeJPEG 将图像编码为 JPEG 格式。
// quality 取值范围为 1～100。
func EncodeJPEG(img image.Image, quality int) ([]byte, error) {
	if quality < 1 || quality > 100 {
		return nil, fmt.Errorf("invalid JPEG quality: %d", quality)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}
