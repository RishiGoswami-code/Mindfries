//go:build windows

package term

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleMode         = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBuffer = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

const (
	enableProcessedInput = 0x0001
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
	enableVTInput        = 0x0200

	enableProcessedOutput = 0x0001
	enableVTProcessing    = 0x0004
)

func getMode(fd uintptr) (uint32, error) {
	var m uint32
	err := syscall.GetConsoleMode(syscall.Handle(fd), &m)
	return m, err
}

func setMode(fd uintptr, m uint32) error {
	r, _, err := procSetConsoleMode.Call(fd, uintptr(m))
	if r == 0 {
		return err
	}
	return nil
}

func isTerminal(fd uintptr) bool {
	_, err := getMode(fd)
	return err == nil
}

func makeRaw(fd uintptr) (func(), error) {
	old, err := getMode(fd)
	if err != nil {
		return nil, err
	}
	raw := old&^(enableProcessedInput|enableLineInput|enableEchoInput) | enableVTInput
	if err := setMode(fd, raw); err != nil {
		return nil, err
	}
	return func() { _ = setMode(fd, old) }, nil
}

func enableANSI(fd uintptr) (func(), error) {
	old, err := getMode(fd)
	if err != nil {
		return nil, err
	}
	if err := setMode(fd, old|enableProcessedOutput|enableVTProcessing); err != nil {
		return nil, err
	}
	return func() { _ = setMode(fd, old) }, nil
}

type coord struct{ x, y int16 }

type smallRect struct{ left, top, right, bottom int16 }

type screenBufferInfo struct {
	size       coord
	cursor     coord
	attributes uint16
	window     smallRect
	maxSize    coord
}

func width(fd uintptr) (int, error) {
	var info screenBufferInfo
	r, _, err := procGetConsoleScreenBuffer.Call(fd, uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 0, err
	}
	return int(info.window.right-info.window.left) + 1, nil
}
