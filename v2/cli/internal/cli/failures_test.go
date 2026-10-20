package cli

// One test per row of the spec's failure table (SPEC §9, "Environment
// failure handling"; v2/PLAN.md task 1.11): each has a clear message and a
// way to recover.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
)

func TestFailureGitMissing(t *testing.T) {
	h := newHarness(t)
	delete(h.machine.Programs, "git")
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()})
	dir := wsDir(t)
	exit, out := h.run("", "start", code, "--dir", dir, "--yes")
	for _, want := range []string{"FAIL  Git", "sudo apt install git", "run 'mindfries start' again", "Your session is kept", "none of this counts against you"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if exit == 0 {
		t.Fatal("setup went ahead without Git")
	}
	// Recovery: install Git, run start again (no code needed).
	h.machine.Programs["git"] = "/usr/bin/git"
	if exit, out := h.run("", "start", "--dir", dir, "--yes"); exit != 0 || !strings.Contains(out, "Workspace ready") {
		t.Fatalf("after installing Git: %d\n%s", exit, out)
	}
}

func TestFailureIDEMissing(t *testing.T) {
	h := newHarness(t)
	delete(h.machine.Programs, "zed")
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()})
	exit, out := h.run("", "start", code, "--dir", wsDir(t), "--yes")
	if exit == 0 || !strings.Contains(out, "no supported editor is installed") || !strings.Contains(out, "Your time hasn't started") {
		t.Fatalf("%d\n%s", exit, out)
	}
	// Nothing started: the session is still waiting to begin.
	if s, _ := h.srv.SessionInfo(h.srv.OnlySessionID()); s.Status != "ready" {
		t.Fatalf("session %s", s.Status)
	}
}

func TestFailureDependencyInstall(t *testing.T) {
	h := newHarness(t)
	task := quickTask()
	task.SetupCommands = []string{"git definitely-not-a-git-command"}
	// Run it, it fails, try again (y), fails again, then skip (n).
	exit, out := h.run("y\ny\nn\n", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{Task: task}), "--dir", wsDir(t))
	if exit != 0 {
		t.Fatalf("a failed install stopped setup: %d\n%s", exit, out)
	}
	if strings.Count(out, "That failed") != 2 || !strings.Contains(out, "setup-1.log") || !strings.Contains(out, "doesn't count against you") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestFailureDownload(t *testing.T) {
	h := newHarness(t)
	h.srv.FailArchive(10) // more than the retries
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()})
	dir := wsDir(t)
	exit, out := h.run("", "start", code, "--dir", dir, "--yes")
	if exit == 0 || !strings.Contains(out, "couldn't download the task") || !strings.Contains(out, "run 'mindfries start' again") {
		t.Fatalf("download failure: %d\n%s", exit, out)
	}
	if _, err := state.Load(h.home); err != nil {
		t.Fatal("the session was lost after a failed download")
	}
	h.srv.FailArchive(0)
	if exit, out := h.run("", "start", "--dir", dir, "--yes"); exit != 0 || !strings.Contains(out, "Workspace ready") {
		t.Fatalf("retry: %d\n%s", exit, out)
	}
}

func TestFailureNetworkLoss(t *testing.T) {
	h := newHarness(t)
	h.keepWatching = true
	dir := wsDir(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()})
	done := make(chan string, 1)
	go func() { _, out := h.run("", "start", code, "--dir", dir, "--yes"); done <- out }()
	waitFor(t, func() bool { _, out := h.run("", "status"); return strings.Contains(out, "last synced") })
	h.srv.SetOffline(true)
	writeFile(t, dir, "limiter.py", "def allow(k):\n    return 1\n")
	waitFor(t, func() bool { _, out := h.run("", "status"); return strings.Contains(out, "offline since") })
	h.srv.SetOffline(false)
	waitFor(t, func() bool {
		_, out := h.run("", "status")
		return strings.Contains(out, "last synced") && !strings.Contains(out, "waiting to send")
	})
	h.srv.EndSession(h.srv.OnlySessionID())
	out := <-done
	if !strings.Contains(out, "Can't reach Mindfries. Keep working") || !strings.Contains(out, "Back online") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestFailureIDECrash(t *testing.T) {
	h := newHarness(t)
	h.machine.Programs["code"] = "/usr/bin/code"
	h.keepWatching = true
	dir := wsDir(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()})
	done := make(chan string, 1)
	go func() { _, out := h.run("", "start", code, "--dir", dir, "--yes"); done <- out }()
	waitFor(t, func() bool { _, out := h.run("", "status"); return strings.Contains(out, "Watcher      running") })
	writeFile(t, dir, "limiter.py", "work in progress\n")
	// The editor crashed; running start again in another terminal reopens
	// it and leaves the running watcher alone.
	exit, out := h.run("", "start")
	if exit != 0 || !strings.Contains(out, "Opened the workspace in VS Code") || !strings.Contains(out, "already running") {
		t.Fatalf("reopen: %d\n%s", exit, out)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "limiter.py")); string(b) != "work in progress\n" {
		t.Fatal("work was lost")
	}
	h.srv.EndSession(h.srv.OnlySessionID())
	<-done
}

func TestFailureWatcherCrash(t *testing.T) {
	h := newHarness(t)
	h.started(fakeapi.SessionOptions{Task: quickTask()})
	// Fake a watcher that died a minute ago.
	dir := state.Dir(h.home)
	b, _ := json.Marshal(watcher.Status{PID: 999, UpdatedAt: time.Now().Add(-time.Minute)})
	os.WriteFile(filepath.Join(dir, "watcher.json"), b, 0o600)
	os.WriteFile(filepath.Join(dir, "watcher.lock"), []byte("999"), 0o600)
	exit, out := h.run("", "start")
	if exit != 0 || !strings.Contains(out, "stopped unexpectedly") || !strings.Contains(out, "recorded as a gap") {
		t.Fatalf("%d\n%s", exit, out)
	}
	var gap bool
	for _, e := range h.srv.Events(h.srv.OnlySessionID()) {
		gap = gap || (e.Type == "watcher.gap" && strings.Contains(string(e.Data), `"crash"`))
	}
	if !gap {
		t.Fatal("no crash gap recorded")
	}
}

func TestFailureUnsupportedOS(t *testing.T) {
	h := newHarness(t)
	a := h.app("", nil)
	var out strings.Builder
	a.Stdout, a.Stderr = &out, &out
	a.Sys = h.machine.Sys("freebsd", "amd64")
	exit := a.Run([]string{"start", h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()}), "--dir", wsDir(t), "--yes"})
	if exit == 0 || !strings.Contains(out.String(), "Contact the hiring team before your assessment") {
		t.Fatalf("%d\n%s", exit, out.String())
	}
}

// The last row, a coding agent still open at start, is
// TestAgentOpenAtStart in watch_test.go.

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timed out")
}
