// Package verify is the verification harness (v2/codebase-engine PLAN task 1.5):
// given a product and a set of task layers (overlay, solution, acceptance), it
// assembles a throwaway workspace and runs the product's check command. It is
// how we prove — because we own the product — that a feature task's reference
// solution passes and a debug task's planted bug fails then greens.
//
// The harness never imports a product; it shells out to the product's own check
// command (e.g. `go test ./...`), so it works for any product stack.
package verify

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Spec describes one verification run.
type Spec struct {
	ProductDir string   // the product repo root (the golden commit, checked out)
	Overlays   []string // directories copied over the product in order (task overlay, solution, acceptance)
	Check      []string // the command to run in the assembled workspace, e.g. ["go","test","./..."]
}

// Result is the outcome. A failing check is a Result (Passed=false), not an
// error; errors are reserved for the harness itself failing (couldn't copy, no
// check command, etc.).
type Result struct {
	Passed bool
	Output string
}

// Run assembles ProductDir + Overlays in a temp dir and runs Check there.
func Run(spec Spec) (Result, error) {
	if len(spec.Check) == 0 {
		return Result{}, fmt.Errorf("verify: no check command")
	}
	work, err := os.MkdirTemp("", "verify-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(work)

	// The product, minus its task scaffolding (candidates get the product plus
	// one task's overlay, never the whole tasks/ tree).
	if err := copyTree(spec.ProductDir, work, map[string]bool{".git": true, "tasks": true}); err != nil {
		return Result{}, fmt.Errorf("verify: copy product: %w", err)
	}
	for _, ov := range spec.Overlays {
		if err := copyTree(ov, work, nil); err != nil {
			return Result{}, fmt.Errorf("verify: apply %s: %w", ov, err)
		}
	}

	cmd := exec.Command(spec.Check[0], spec.Check[1:]...)
	cmd.Dir = work
	out, err := cmd.CombinedOutput()
	return Result{Passed: err == nil, Output: string(out)}, nil
}

// copyTree copies src into dst. Top-level directory names in skip are pruned.
func copyTree(src, dst string, skip map[string]bool) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() && skip[rel] {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
