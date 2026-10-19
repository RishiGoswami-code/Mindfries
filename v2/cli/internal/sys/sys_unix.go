//go:build linux || darwin

package sys

import (
	"context"
	"strings"
	"syscall"
)

func diskFree(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}

// processes lists process names with ps. Only the command name column is
// read: never arguments, users or anything else.
func processes(ctx context.Context) ([]string, error) {
	out, err := output(ctx, "", "ps", "-A", "-o", "comm=")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// macOS prints the full path of the executable.
		if i := strings.LastIndex(l, "/"); i >= 0 {
			l = l[i+1:]
		}
		names = append(names, l)
	}
	return names, nil
}
