package main

import (
	"fmt"
	"time"

	"keyframe/internal"
)

func main() {
	start := time.Now()

	img, err := internal.Capture(0)
	if err != nil {
		panic(err)
	}
	captureTime := time.Since(start)

	bounds := img.Bounds()
	rawSize := len(img.Pix)

	fmt.Printf("Original resolution: %d x %d\n", bounds.Dx(), bounds.Dy())
	fmt.Printf("Raw RGBA: %.2f MiB\n", float64(rawSize)/(1024*1024))
	fmt.Printf("Capture time: %v\n", captureTime)

	for _, scale := range []float64{1.0, 0.75, 0.5, 0.25} {
		start = time.Now()

		scaled, err := internal.ScaleImage(img, scale)
		if err != nil {
			panic(err)
		}
		scaleTime := time.Since(start)

		b := scaled.Bounds()

		fmt.Printf(
			"\nScale=%.2f | Resolution=%dx%d | Resize=%v\n",
			scale, b.Dx(), b.Dy(), scaleTime,
		)

		for _, quality := range []int{100, 90, 80, 70, 60, 50} {
			start = time.Now()

			data, err := internal.EncodeJPEG(scaled, quality)
			if err != nil {
				panic(err)
			}

			encodeTime := time.Since(start)

			fmt.Printf(
				"  Quality=%3d | Size=%8.2f KiB | Ratio=%6.2f%% | Encode=%v\n",
				quality,
				float64(len(data))/1024,
				float64(len(data))/float64(rawSize)*100,
				encodeTime,
			)
		}
	}
}
