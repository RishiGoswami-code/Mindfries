package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// unpackBundle must reconstruct the candidate's workspace from a git bundle.
func TestUnpackBundle(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "first")

	head, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(t.TempDir(), "b.bundle")
	if out, err := exec.Command("git", "-C", repo, "bundle", "create", bundlePath, "--all").CombinedOutput(); err != nil {
		t.Fatalf("bundle: %v: %s", err, out)
	}
	bundle, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}

	dir, cleanup, err := unpackBundle(bundle, strings.TrimSpace(string(head)))
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "hello.txt")); err != nil || string(b) != "hi" {
		t.Fatalf("unpacked file wrong: %q %v", b, err)
	}
}
