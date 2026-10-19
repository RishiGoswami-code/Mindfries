package sys

import (
	"context"
	"errors"
	"os"
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
	// Dirs maps a directory to the names in it; Data maps a file to its
	// contents.
	Dirs map[string][]string
	Data map[string]string
	// Reads records every Output, ReadDir and ReadFile call, so tests can
	// check nothing else is looked at.
	Reads []string
}

func (f *Fake) record(s string) {
	f.mu.Lock()
	f.Reads = append(f.Reads, s)
	f.mu.Unlock()
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
			f.record("run " + key)
			if out, ok := f.Outputs[key]; ok {
				return out, nil
			}
			return "", errors.New("fake: no output scripted for " + key)
		},
		DiskFree: func(string) (uint64, error) { return f.Free, nil },
		ReadDir: func(p string) ([]string, error) {
			f.record("readdir " + p)
			if d, ok := f.Dirs[p]; ok {
				return d, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFile: func(p string) ([]byte, error) {
			f.record("read " + p)
			if d, ok := f.Data[p]; ok {
				return []byte(d), nil
			}
			return nil, os.ErrNotExist
		},
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
