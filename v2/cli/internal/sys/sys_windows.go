//go:build windows

package sys

import (
	"context"
	"encoding/csv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func diskFree(path string) (uint64, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var avail, total, free uint64
	r, _, e := procGetDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if r == 0 {
		return 0, e
	}
	return avail, nil
}

// processes lists image names with tasklist. Only the image name column is
// kept.
func processes(ctx context.Context) ([]string, error) {
	out, err := output(ctx, "", "tasklist", "/fo", "csv", "/nh")
	if err != nil {
		return nil, err
	}
	recs, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, r := range recs {
		if len(r) > 0 && r[0] != "" {
			names = append(names, r[0])
		}
	}
	return names, nil
}
