package grade

import (
	"os"
	"path/filepath"
	"testing"
)

func p1(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd() // internal/grade
	d := filepath.Clean(filepath.Join(wd, "../../../products/p1-support-saas"))
	if _, err := os.Stat(filepath.Join(d, "go.mod")); err != nil {
		t.Skipf("P1 not found at %s", d)
	}
	return d
}

// A candidate who fixed the bug looks like the clean product (its routing.go is
// the correct, fixed version). Grading must pass.
func TestGradePassesAFixedSubmission(t *testing.T) {
	res, err := Run(p1(t), p1(t), "debug-auto-assign")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatalf("fixed submission should pass:\n%s", res.Output)
	}
}

// A candidate who left the bug in = the product with the planted-bug overlay.
// Grading must fail.
func TestGradeFailsABuggySubmission(t *testing.T) {
	cand := t.TempDir()
	copyTree(t, p1(t), cand, map[string]bool{".git": true, "_tasks": true})
	copyTree(t, filepath.Join(p1(t), "_tasks/debug-auto-assign/overlay"), cand, nil)

	res, err := Run(cand, p1(t), "debug-auto-assign")
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed {
		t.Fatalf("buggy submission should fail grading; output:\n%s", res.Output)
	}
}

func copyTree(t *testing.T, src, dst string, skip map[string]bool) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if skip[rel] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dst, rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
