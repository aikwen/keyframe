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
	user32               = syscall.NewLazyDLL("user32.dll")
	getCursorPos         = user32.NewProc("GetCursorPos")
	getPhysicalCursorPos = user32.NewProc("GetPhysicalCursorPos")
)

type point struct {
	X int32
	Y int32
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
	const (
		display   = 0
		threshold = 20
	)

	fmt.Println("Cursor edge diagnostic")
	fmt.Println("Move the cursor to each screen edge. Ctrl+C to stop.")
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

		count := screenshot.NumActiveDisplays()
		if display >= count {
			panic("invalid display index")
		}

		bounds := screenshot.GetDisplayBounds(display)

		left := physical.X < bounds.Min.X+threshold
		right := physical.X >= bounds.Max.X-threshold
		top := physical.Y < bounds.Min.Y+threshold
		bottom := physical.Y >= bounds.Max.Y-threshold

		fmt.Printf(
			"Logical=%v Physical=%v Bounds=%v | L=%t R=%t T=%t B=%t\n",
			logical,
			physical,
			bounds,
			left,
			right,
			top,
			bottom,
		)

		time.Sleep(500 * time.Millisecond)
	}
}
