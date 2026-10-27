package internal

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"

	"github.com/kbinani/screenshot"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	// GetPhysicalCursorPos and GetCursorPos
	procGetCursorPos = user32.NewProc("GetPhysicalCursorPos")
)

type point struct {
	X int32
	Y int32
}

func EnableDPIAwareness() error {
	proc := user32.NewProc("SetProcessDpiAwarenessContext")

	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	ret, _, err := proc.Call(^uintptr(3))
	if ret == 0 {
		return fmt.Errorf("SetProcessDpiAwarenessContext failed: %w", err)
	}
	return nil
}

// CursorPosition 获取当前鼠标的屏幕坐标。
func CursorPosition() (image.Point, error) {
	var p point

	ret, _, err := procGetCursorPos.Call(
		uintptr(unsafe.Pointer(&p)),
	)
	if ret == 0 {
		return image.Point{}, fmt.Errorf("GetCursorPos failed: %w", err)
	}

	return image.Pt(int(p.X), int(p.Y)), nil
}

// IsCursorAtEdge 判断鼠标是否位于指定显示器的边缘区域。
// threshold 表示边缘区域的宽度，单位为像素。
func IsCursorAtEdge(display, threshold int) (bool, error) {
	if threshold <= 0 {
		return false, fmt.Errorf("invalid threshold: %d", threshold)
	}

	count := screenshot.NumActiveDisplays()
	if display < 0 || display >= count {
		return false, fmt.Errorf("invalid display index: %d", display)
	}

	bounds := screenshot.GetDisplayBounds(display)

	pos, err := CursorPosition()
	if err != nil {
		return false, err
	}

	// 鼠标不在目标显示器内。
	if !pos.In(bounds) {
		return false, nil
	}

	return pos.X < bounds.Min.X+threshold ||
			pos.X >= bounds.Max.X-threshold ||
			pos.Y < bounds.Min.Y+threshold ||
			pos.Y >= bounds.Max.Y-threshold,
		nil
}
