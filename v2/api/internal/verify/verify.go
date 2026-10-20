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

	// Isolate runs the check with a stripped environment (see isolatedEnv)
	// instead of this process's own: for code we don't trust, such as a
	// candidate's submission, which must never see the grader's secrets.
	Isolate bool
	// ExtraEnv is added to the isolated environment (KEY=value).
	ExtraEnv []string
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
	if err := copyTree(spec.ProductDir, work, map[string]bool{".git": true, "_tasks": true}); err != nil {
		return Result{}, fmt.Errorf("verify: copy product: %w", err)
	}
	for _, ov := range spec.Overlays {
		if err := copyTree(ov, work, nil); err != nil {
			return Result{}, fmt.Errorf("verify: apply %s: %w", ov, err)
		}
	}

	cmd := exec.Command(spec.Check[0], spec.Check[1:]...)
	cmd.Dir = work
	if spec.Isolate {
		cmd.Env = isolatedEnv(work, spec.ExtraEnv)
	}
	out, err := cmd.CombinedOutput()
	return Result{Passed: err == nil, Output: string(out)}, nil
}

// passEnv are the variables an isolated check may inherit: where to find the
// toolchain and the bare minimum a process needs to start. Nothing else comes
// through, so credentials in the grader's environment never reach the
// candidate's code.
var passEnv = []string{
	"PATH", "SystemRoot", "SYSTEMROOT", "SystemDrive", "ComSpec", "PATHEXT", "WINDIR",
	"GOROOT", "GOPATH", "GOMODCACHE", "GOFLAGS", "GOPROXY", "GOCACHE",
	"JAVA_HOME", "NVM_DIR", "PYTHONHOME", "LANG", "LC_ALL", "TZ",
}

// isolatedEnv is the stripped environment: the allow-listed variables, a HOME
// and temp folder inside the throwaway workspace, and any extras.
func isolatedEnv(work string, extra []string) []string {
	env := []string{}
	for _, k := range passEnv {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	// Go finds its caches relative to HOME, which is about to change: pin them to
	// the grader's own (read-only use of already-downloaded modules, and a warm
	// build cache), so a check doesn't need the network or a cold build.
	has := func(k string) bool { _, ok := os.LookupEnv(k); return ok }
	if realHome, err := os.UserHomeDir(); err == nil {
		if !has("GOPATH") {
			env = append(env, "GOPATH="+filepath.Join(realHome, "go"))
		}
	}
	if !has("GOCACHE") {
		if c, err := os.UserCacheDir(); err == nil {
			env = append(env, "GOCACHE="+filepath.Join(c, "go-build"))
		}
	}
	home := filepath.Join(work, ".home")
	_ = os.MkdirAll(home, 0o755)
	env = append(env, "HOME="+home, "USERPROFILE="+home, "TMPDIR="+home, "TEMP="+home, "TMP="+home, "CI=1")
	return append(env, extra...)
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
