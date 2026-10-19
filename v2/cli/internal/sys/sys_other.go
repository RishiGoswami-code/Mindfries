//go:build !linux && !darwin && !windows

package sys

import "context"

func diskFree(string) (uint64, error) { return 0, ErrUnsupported }

func processes(context.Context) ([]string, error) { return nil, ErrUnsupported }
