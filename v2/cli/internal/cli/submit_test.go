package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/state"
)

// quickTask is the demo task with a check that runs anywhere.
func quickTask() *fakeapi.Task {
	t := fakeapi.DemoTask()
	t.CheckCommands = []string{"git --version"}
	return t
}

func TestSubmitHandsIn(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask()})
	id := h.srv.OnlySessionID()
	os.WriteFile(filepath.Join(dir, "limiter.py"), []byte("def allow(key):\n    return True\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("Design notes\n"), 0o644)

	exit, out := h.run("", "submit", "-m", "Rate limiter", "--yes")
	if exit != 0 {
		t.Fatalf("submit: %d\n%s", exit, out)
	}
	for _, want := range []string{"Running the task's check: git --version", "limiter.py", "notes.md", "Submitted. Commit"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	sub, bundle := h.srv.Submission(id)
	if sub == nil || sub.Status != api.SubmissionVerified || len(bundle) == 0 {
		t.Fatalf("server submission: %+v (%d bytes)", sub, len(bundle))
	}
	if info, _ := h.srv.SessionInfo(id); info.Status != api.StatusSubmitted {
		t.Fatalf("session status %s", info.Status)
	}
	// The local session is gone, the code and a record of the hand-in stay.
	if _, err := state.Load(h.home); err != state.ErrNone {
		t.Error("local session left behind")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "limiter.py")); !strings.Contains(string(b), "return True") {
		t.Error("the candidate's code was changed")
	}
	if fi, _ := os.Stat(filepath.Join(dir, "TASK.md")); fi.Mode().Perm()&0o200 == 0 {
		t.Error("TASK.md is still read-only after the session")
	}
	_, out = h.run("", "status")
	if !strings.Contains(out, "Last submitted") || !strings.Contains(out, sub.ID) {
		t.Fatalf("status after submit:\n%s", out)
	}
}

func TestMismatchedBundleIsRefused(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask()})
	os.WriteFile(filepath.Join(dir, "limiter.py"), []byte("def allow(key):\n    return False\n"), 0o644)
	a := h.app("", nil)
	l, c, err := a.active()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "work")
	m, bundle, err := a.buildSubmission(ctx, l, state.Dir(h.home), "work", nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Files[0].SHA256 = strings.Repeat("0", 64) // claims a different file
	sub, err := c.CreateSubmission(ctx, *m)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(bundle)
	defer f.Close()
	_, err = c.UploadBundle(ctx, sub.ID, f, m.BundleBytes)
	p, ok := api.AsProblem(err)
	if !ok || p.Code != api.CodeBundleMismatch || len(p.Mismatches) == 0 || p.Mismatches[0].What != "file_hash" {
		t.Fatalf("mismatch not refused: %v %+v", err, p)
	}
	if info, _ := h.srv.SessionInfo(h.srv.OnlySessionID()); info.Status != api.StatusActive {
		t.Fatal("a refused submission ended the session")
	}
}

func TestSubmitWhileWatching(t *testing.T) {
	h := newHarness(t)
	h.keepWatching = true
	dir := wsDir(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()})
	done := make(chan string, 1)
	go func() { _, out := h.run("", "start", code, "--dir", dir, "--yes"); done <- out }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, out := h.run("", "status"); strings.Contains(out, "Watcher      running") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	os.WriteFile(filepath.Join(dir, "limiter.py"), []byte("def allow(key):\n    return key != ''\n"), 0o644)
	time.Sleep(100 * time.Millisecond)
	if exit, out := h.run("", "submit", "-m", "done", "--yes"); exit != 0 {
		t.Fatalf("submit: %d\n%s", exit, out)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("start's watcher didn't stop after submit")
	}
	evs := h.srv.Events(h.srv.OnlySessionID())
	last := evs[len(evs)-1]
	if last.Type != "watcher.stopped" || !strings.Contains(string(last.Data), "submitted") {
		t.Fatalf("last event %s %s", last.Type, last.Data)
	}
	var sawChange bool
	for _, e := range evs {
		sawChange = sawChange || (e.Type == "file.changed" && strings.Contains(string(e.Data), "limiter.py"))
	}
	if !sawChange {
		t.Fatal("the last change before submitting wasn't recorded")
	}
}

func TestSubmitWithNoChanges(t *testing.T) {
	h := newHarness(t)
	h.started(fakeapi.SessionOptions{Task: quickTask()})
	exit, out := h.run("n\n", "submit", "-m", "x")
	if exit == 0 || !strings.Contains(out, "Nothing has changed") {
		t.Fatalf("no changes: %d\n%s", exit, out)
	}
	if sub, _ := h.srv.Submission(h.srv.OnlySessionID()); sub != nil {
		t.Fatal("submitted without confirmation")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := (gitRepo(dir)).Run(context.Background(), args...); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}
