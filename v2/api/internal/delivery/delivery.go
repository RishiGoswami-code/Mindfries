// Package delivery assembles the candidate workspace for a resolved task
// (v2/codebase-engine PLAN task 2.4). It turns a product (at its golden commit)
// plus a task into exactly the files a candidate receives: the product source,
// minus its task scaffolding, with the task's overlay applied and TASK.md set
// to the resolved brief and locked. The hidden acceptance tests and the
// reference solution live under the product's tasks/ tree and are deliberately
// excluded — a candidate never receives the answer key.
//
// This is the authoritative server-side assembly the real /session/bundle
// serves. The CLI's fake API mirrors the same shape (see v2/cli/fakeapi).
package delivery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mindfries/v2/api/internal/taskgen"
)

// Workspace is the candidate's starting files plus the metadata the CLI needs
// to set up Git, lock TASK.md, and run checks. It maps directly onto the fake
// API's Task and the real bundle manifest.
type Workspace struct {
	Files       map[string]string // path -> content
	Locked      []string          // paths the candidate may not edit (TASK.md)
	Check       []string          // the command that verifies a submission
	Runtimes    []string          // required runtimes, e.g. ["go"]
	Branch      string            // the assessment branch
	TaskVersion string            // the pinned task version this workspace is for
}

const assessmentBranch = "mindfries/assessment"

// Assemble builds the workspace for one task of a product. taskID is the bank
// task directory name; brief becomes TASK.md; taskVersion pins the session.
func Assemble(productDir, taskID, brief, taskVersion string) (Workspace, error) {
	files, err := readTree(productDir, map[string]bool{".git": true, "_tasks": true})
	if err != nil {
		return Workspace{}, fmt.Errorf("delivery: read product: %w", err)
	}

	// Apply the task's overlay (a feature stub, or a planted bug for a debug
	// task) over the product. Never the acceptance/ or solution/ dirs.
	overlayDir := filepath.Join(productDir, "_tasks", taskID, "overlay")
	if _, err := os.Stat(overlayDir); err == nil {
		overlay, err := readTree(overlayDir, nil)
		if err != nil {
			return Workspace{}, fmt.Errorf("delivery: read overlay: %w", err)
		}
		for p, c := range overlay {
			files[p] = c
		}
	}

	files["TASK.md"] = brief

	check, err := readCheck(filepath.Join(productDir, "_tasks", taskID, "task.json"))
	if err != nil {
		return Workspace{}, err
	}

	ws := Workspace{
		Files:       files,
		Locked:      []string{"TASK.md"},
		Check:       check,
		Runtimes:    detectRuntimes(check, files),
		Branch:      assessmentBranch,
		TaskVersion: taskVersion,
	}
	return ws, nil
}

// FromResolved assembles the workspace for a taskgen.ResolvedTask.
func FromResolved(productDir string, rt taskgen.ResolvedTask) (Workspace, error) {
	return Assemble(productDir, rt.BankTaskID, rt.Brief, rt.TaskVersion)
}

// WriteTo materializes the workspace's files under dir (for the real bundler,
// and for tests that run the check command).
func (w Workspace) WriteTo(dir string) error {
	for p, content := range w.Files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Build/VCS directories are never part of a delivered workspace, at any depth:
// a candidate installs their own dependencies. Pruned by basename everywhere,
// so a product with a JS console doesn't ship its node_modules.
var prunedDirs = map[string]bool{".git": true, "node_modules": true, "dist": true}

// readTree reads every regular file under root into a path->content map,
// pruning top-level directories named in skip and build/VCS dirs at any depth.
func readTree(root string, skip map[string]bool) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if skip[rel] || prunedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	return files, err
}

// readCheck returns the task's check command from its task.json.
func readCheck(taskJSON string) ([]string, error) {
	b, err := os.ReadFile(taskJSON)
	if err != nil {
		return nil, fmt.Errorf("delivery: read task.json: %w", err)
	}
	var tf struct {
		Check []string `json:"check"`
	}
	if err := json.Unmarshal(b, &tf); err != nil {
		return nil, fmt.Errorf("delivery: parse task.json: %w", err)
	}
	return tf.Check, nil
}

// detectRuntimes returns the runtimes the candidate must have installed to run
// the task. It keys off the task's check command — what actually runs — not the
// delivered files: a product can ship several languages as context (P1 ships
// its Go API next to the React console) while a given task's check uses only
// one, so a frontend task must not demand Go just because a go.mod is present.
func detectRuntimes(check []string, files map[string]string) []string {
	cmd := strings.ToLower(strings.Join(check, " "))
	switch {
	case containsAny(cmd, "npm", "pnpm", "yarn", "vitest", "jest", "node"):
		return []string{"node"}
	case containsAny(cmd, "python", "pytest", "unittest"):
		return []string{"python"}
	case goWord.MatchString(cmd):
		return []string{"go"}
	}
	// Fallback when the check doesn't name a known runtime: infer from files.
	if _, ok := files["go.mod"]; ok {
		return []string{"go"}
	}
	for p := range files {
		if strings.HasSuffix(p, ".py") {
			return []string{"python"}
		}
	}
	return nil
}

var goWord = regexp.MustCompile(`\bgo\b`)

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
