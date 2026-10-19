package sys

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
)

// Fake is a scripted machine for tests.
type Fake struct {
	mu sync.Mutex
	// Programs maps a program name to its full path.
	Programs map[string]string
	// Files that exist.
	Files map[string]bool
	// Outputs maps "name arg1 arg2" to the output; a missing key fails.
	Outputs map[string]string
	Env     map[string]string
	Free    uint64
	Procs   []string
	// Started records every Start call as "name arg1 arg2".
	Started []string
	// StartErr makes Start fail.
	StartErr error
}

// Sys returns a *Sys backed by f.
func (f *Fake) Sys(goos, goarch string) *Sys {
	return &Sys{
		GOOS:   goos,
		GOARCH: goarch,
		Getenv: func(k string) string { return f.Env[k] },
		LookPath: func(n string) (string, error) {
			if p, ok := f.Programs[n]; ok {
				return p, nil
			}
			return "", exec.ErrNotFound
		},
		Exists: func(p string) bool { return f.Files[p] },
		Output: func(_ context.Context, _ string, name string, args ...string) (string, error) {
			key := strings.Join(append([]string{name}, args...), " ")
			if out, ok := f.Outputs[key]; ok {
				return out, nil
			}
			return "", errors.New("fake: no output scripted for " + key)
		},
		DiskFree: func(string) (uint64, error) { return f.Free, nil },
		Processes: func(context.Context) ([]string, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			return append([]string(nil), f.Procs...), nil
		},
		Start: func(name string, args ...string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.StartErr != nil {
				return f.StartErr
			}
			f.Started = append(f.Started, strings.Join(append([]string{name}, args...), " "))
			return nil
		},
	}
}

// SetProcs replaces the running processes.
func (f *Fake) SetProcs(p ...string) {
	f.mu.Lock()
	f.Procs = p
	f.mu.Unlock()
}
