package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/state"
)

func commits(t *testing.T, dir string) int {
	t.Helper()
	out, err := (&gitx.Repo{Dir: dir}).Line(context.Background(), "rev-list", "--count", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(out)
	return n
}

func TestWorkspaceSetUp(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{})
	for _, f := range []string{"TASK.md", "limiter.py", "test_limiter.py", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s missing: %v", f, err)
		}
	}
	r := &gitx.Repo{Dir: dir}
	ctx := context.Background()
	if b, _ := r.Branch(ctx); b != "mindfries/assessment" {
		t.Errorf("branch = %q", b)
	}
	if name, _ := r.Line(ctx, "config", "user.name"); name != "Candidate" {
		t.Errorf("local identity = %q", name)
	}
	if n, _ := r.ChangedFiles(ctx); n != 0 {
		t.Errorf("baseline left %d files uncommitted", n)
	}
	l, err := state.Load(h.home)
	if err != nil {
		t.Fatal(err)
	}
	head, _ := r.Head(ctx)
	if l.BaseCommit != head || l.TaskVersion != "demo-1.0.0" || l.Workspace != dir {
		t.Errorf("state: %+v", l)
	}
	if len(l.Locked) != 1 || l.Locked[0].Path != "TASK.md" {
		t.Fatalf("locked: %+v", l.Locked)
	}
	fi, _ := os.Stat(filepath.Join(dir, "TASK.md"))
	if fi.Mode().Perm()&0o222 != 0 {
		t.Errorf("TASK.md is writable: %v", fi.Mode())
	}
}

func TestRerunResumesWithoutDuplicating(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{})
	exit, out := h.run("", "start")
	if exit != 0 || !strings.Contains(out, "Carrying on") || !strings.Contains(out, "Workspace: "+dir) {
		t.Fatalf("re-run: %d\n%s", exit, out)
	}
	if strings.Contains(out, "Downloading") {
		t.Fatal("re-run downloaded the task again")
	}
	if n := commits(t, dir); n != 1 {
		t.Fatalf("re-run made more commits: %d", n)
	}
}

func TestTamperedManifestIsRejected(t *testing.T) {
	h := newHarness(t)
	h.srv.TamperManifest(true)
	dir := filepath.Join(t.TempDir(), "ws")
	exit, out := h.run("", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", dir, "--yes")
	if exit == 0 || !strings.Contains(out, "couldn't be verified") {
		t.Fatalf("tampered manifest: %d\n%s", exit, out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("something was written for a tampered manifest")
	}
}

func TestTamperedArchiveIsRejected(t *testing.T) {
	h := newHarness(t)
	h.srv.TamperArchive(true)
	dir := filepath.Join(t.TempDir(), "ws")
	exit, out := h.run("", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", dir, "--yes")
	if exit == 0 || !strings.Contains(out, "doesn't match its signed manifest") {
		t.Fatalf("tampered archive: %d\n%s", exit, out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("something was written for a tampered archive")
	}
}

func TestBrokenDownloadResumes(t *testing.T) {
	h := newHarness(t)
	h.srv.TruncateArchiveOnce()
	dir := filepath.Join(t.TempDir(), "ws")
	exit, out := h.run("", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", dir, "--yes")
	if exit != 0 || !strings.Contains(out, "retrying") {
		t.Fatalf("broken download: %d\n%s", exit, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "limiter.py")); err != nil {
		t.Fatal(err)
	}
}

func TestNonEmptyFolderNeedsConfirmation(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o644)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{})
	// --yes doesn't answer this one; "n" refuses.
	exit, out := h.run("n\n", "start", code, "--dir", dir, "--yes")
	if exit == 0 || !strings.Contains(out, "isn't empty") {
		t.Fatalf("non-empty folder: %d\n%s", exit, out)
	}
	// Same session, now confirmed.
	exit, out = h.run("y\n", "start", "--dir", dir, "--yes")
	if exit != 0 {
		t.Fatalf("confirmed: %d\n%s", exit, out)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "notes.txt")); string(b) != "mine" {
		t.Fatal("the candidate's own file was changed")
	}
}

func TestExistingRepositoryIsRefused(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	exit, out := h.run("", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", dir, "--yes")
	if exit == 0 || !strings.Contains(out, "already a Git repository") {
		t.Fatalf("repo: %d\n%s", exit, out)
	}
}
