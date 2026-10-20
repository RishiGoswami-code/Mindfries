package fakeapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
)

// ProductTask builds the Task the fake serves for a real product assessment:
// the product source (minus its .git and tasks/ scaffolding) with the task's
// overlay applied, TASK.md set to brief and locked, and the check command and
// runtimes from the product. It lets the CLI's end-to-end flow run against a
// real product (e.g. v2/products/p1-support-saas) instead of only DemoTask.
//
// The authoritative copy of this assembly lives in v2/api/internal/delivery
// (the real /session/bundle). This is a thin, owned mirror so the fakeapi
// module stays self-contained — the V2 "copy a thin piece and own it" rule.
func ProductTask(productDir, taskID, version, brief string) (*Task, error) {
	files, err := readTree(productDir, map[string]bool{".git": true, "_tasks": true})
	if err != nil {
		return nil, fmt.Errorf("fakeapi: read product: %w", err)
	}
	overlayDir := filepath.Join(productDir, "_tasks", taskID, "overlay")
	if _, err := os.Stat(overlayDir); err == nil {
		overlay, err := readTree(overlayDir, nil)
		if err != nil {
			return nil, fmt.Errorf("fakeapi: read overlay: %w", err)
		}
		for p, c := range overlay {
			files[p] = c
		}
	}
	files["TASK.md"] = brief

	var tf struct {
		Check []string `json:"check"`
	}
	if b, err := os.ReadFile(filepath.Join(productDir, "_tasks", taskID, "task.json")); err == nil {
		_ = json.Unmarshal(b, &tf)
	}

	var runtimes []api.Runtime
	if _, ok := files["go.mod"]; ok {
		runtimes = []api.Runtime{{Name: "go", Version: ">=1.22", InstallURL: "https://go.dev/dl/"}}
	} else {
		for p := range files {
			if strings.HasSuffix(p, ".py") {
				runtimes = []api.Runtime{{Name: "python", Version: ">=3.10", InstallURL: "https://www.python.org/downloads/"}}
				break
			}
		}
	}

	return &Task{
		Version:       version,
		Files:         files,
		Locked:        []string{"TASK.md"},
		Runtimes:      runtimes,
		CheckCommands: tf.Check,
		Branch:        "mindfries/assessment",
		AuthorName:    "Candidate",
		AuthorEmail:   "candidate@assessment.mindfries.invalid",
	}, nil
}

// readTree reads every regular file under root into a slash-path->content map,
// pruning top-level directories named in skip.
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
			if skip[rel] {
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
