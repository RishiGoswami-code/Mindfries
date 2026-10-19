// Package sys is the CLI's narrow view of the machine: finding and running
// programs, free disk space, the OS. Commands take a *Sys so tests can
// replace any of it.
package sys

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Sys is swappable access to the machine.
type Sys struct {
	GOOS, GOARCH string
	Getenv       func(string) string
	// LookPath finds a program on PATH.
	LookPath func(string) (string, error)
	// Exists reports whether a file or directory exists.
	Exists func(string) bool
	// Output runs a program and returns its combined output.
	Output func(ctx context.Context, dir string, name string, args ...string) (string, error)
	// DiskFree returns the bytes available to the user at path.
	DiskFree func(path string) (uint64, error)
	// Processes returns the names of running processes.
	Processes func(ctx context.Context) ([]string, error)
	// Start launches a program without waiting for it.
	Start func(name string, args ...string) error
	// ReadDir lists the names in a directory; ReadFile reads a file. Used
	// only for editor settings and plugin folders (v2/PLAN.md task 1.8).
	ReadDir  func(string) ([]string, error)
	ReadFile func(string) ([]byte, error)
}

// Real is the actual machine.
func Real() *Sys {
	return &Sys{
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		Getenv:    os.Getenv,
		LookPath:  exec.LookPath,
		Exists:    func(p string) bool { _, err := os.Stat(p); return err == nil },
		Output:    output,
		DiskFree:  diskFree,
		Processes: processes,
		Start:     start,
		ReadDir: func(p string) ([]string, error) {
			ents, err := os.ReadDir(p)
			names := make([]string, 0, len(ents))
			for _, e := range ents {
				names = append(names, e.Name())
			}
			return names, err
		},
		ReadFile: os.ReadFile,
	}
}

func output(ctx context.Context, dir, name string, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	return b.String(), err
}

func start(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// ErrUnsupported is returned where the platform has no implementation.
var ErrUnsupported = errors.New("not supported on this platform")

// Home is the user's home directory, or "." if unknown.
func (s *Sys) Home() string {
	if h := s.Getenv("HOME"); h != "" {
		return h
	}
	if h := s.Getenv("USERPROFILE"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}

// Find looks for a program on PATH and then at the given absolute paths
// (with $VAR expanded). It returns "" if none exists.
func (s *Sys) Find(names []string, paths []string) string {
	for _, n := range names {
		if p, err := s.LookPath(n); err == nil {
			return p
		}
	}
	for _, p := range paths {
		p = os.Expand(p, s.Getenv)
		if p == "" || strings.Contains(p, "$") || !filepath.IsAbs(p) {
			continue
		}
		if s.Exists(p) {
			return p
		}
	}
	return ""
}
