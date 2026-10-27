package internal

import (
	"fmt"
	"image"
	stddraw "image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
)

// ScaleImage 按比例缩放图像。
// 例如 scale=0.5 表示宽和高都缩放到原来的 50%。
// 要求 scale > 0。
func ScaleImage(img image.Image, scale float64) (*image.RGBA, error) {
	if img == nil {
		return nil, fmt.Errorf("nil image")
	}
	if scale <= 0 {
		return nil, fmt.Errorf("invalid scale: %f", scale)
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("invalid source image size: %dx%d", srcW, srcH)
	}

	// scale=1 时直接返回原图（如果本来就是 *image.RGBA）。
	if scale == 1 {
		if rgba, ok := img.(*image.RGBA); ok {
			return rgba, nil
		}

		dst := image.NewRGBA(image.Rect(0, 0, srcW, srcH))
		stddraw.Draw(dst, dst.Bounds(), img, bounds.Min, stddraw.Src)
		return dst, nil
	}

	dstW := int(math.Round(float64(srcW) * scale))
	dstH := int(math.Round(float64(srcH) * scale))

	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	// 使用 CatmullRom，质量较好，适合截图缩放。
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)

	return dst, nil
}
