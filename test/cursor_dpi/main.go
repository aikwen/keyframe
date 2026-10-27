package main

import (
	"fmt"
	"image"
	"syscall"
	"time"
	"unsafe"

	"github.com/kbinani/screenshot"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	getCursorPos         = user32.NewProc("GetCursorPos")
	getPhysicalCursorPos = user32.NewProc("GetPhysicalCursorPos")
	setDPIAwareness      = user32.NewProc("SetProcessDpiAwarenessContext")
)

type point struct {
	X int32
	Y int32
}

func enableDPIAwareness() error {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	ret, _, err := setDPIAwareness.Call(^uintptr(3))
	if ret == 0 {
		return fmt.Errorf("SetProcessDpiAwarenessContext failed: %w", err)
	}
	return nil
}

func cursorPosition(proc *syscall.LazyProc) (image.Point, error) {
	var p point

	ret, _, err := proc.Call(uintptr(unsafe.Pointer(&p)))
	if ret == 0 {
		return image.Point{}, fmt.Errorf("%s failed: %w", proc.Name, err)
	}

	return image.Pt(int(p.X), int(p.Y)), nil
}

func main() {
	// 必须在调用显示器相关 API 之前设置 DPI Awareness。
	if err := enableDPIAwareness(); err != nil {
		fmt.Println("DPI Awareness:", err)
	} else {
		fmt.Println("DPI Awareness: Per-Monitor V2 enabled")
	}

	const (
		display   = 0
		threshold = 20
	)

	count := screenshot.NumActiveDisplays()
	if display >= count {
		panic("invalid display index")
	}

	bounds := screenshot.GetDisplayBounds(display)

	fmt.Printf("Display bounds: %v\n", bounds)
	fmt.Println("Move cursor to each screen edge. Ctrl+C to stop.")
	fmt.Println()

	for {
		logical, err := cursorPosition(getCursorPos)
		if err != nil {
			panic(err)
		}

		physical, err := cursorPosition(getPhysicalCursorPos)
		if err != nil {
			panic(err)
		}

		inside := physical.In(bounds)

		left := inside && physical.X < bounds.Min.X+threshold
		right := inside && physical.X >= bounds.Max.X-threshold
		top := inside && physical.Y < bounds.Min.Y+threshold
		bottom := inside && physical.Y >= bounds.Max.Y-threshold

		fmt.Printf(
			"Logical=%v Physical=%v | L=%t R=%t T=%t B=%t\n",
			logical,
			physical,
			left,
			right,
			top,
			bottom,
		)

		time.Sleep(500 * time.Millisecond)
	}
}
