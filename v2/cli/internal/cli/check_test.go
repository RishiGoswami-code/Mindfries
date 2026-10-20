package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
)

// checkEvents are the check.run events the fake API has stored.
func checkEvents(h *harness) []checkEvent {
	var out []checkEvent
	for _, e := range h.srv.Events(h.srv.OnlySessionID()) {
		var ev checkEvent
		if e.Type == "check.run" && json.Unmarshal(e.Data, &ev) == nil {
			out = append(out, ev)
		}
	}
	return out
}

// watching starts a session whose watcher keeps running, and returns a
// function that ends it and waits for the watcher to send everything.
func watching(t *testing.T, h *harness, task *fakeapi.Task) func() {
	t.Helper()
	h.keepWatching = true
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: task})
	done := make(chan string, 1)
	go func() { _, out := h.run("", "start", code, "--dir", wsDir(t), "--yes"); done <- out }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, out := h.run("", "status"); strings.Contains(out, "Watcher      running") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return func() {
		if exit, out := h.run("", "stop", "--yes"); exit != 0 {
			t.Fatalf("stop: %d\n%s", exit, out)
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("start's watcher didn't stop")
		}
	}
}

func TestCheckRecordsTheRun(t *testing.T) {
	h := newHarness(t)
	end := watching(t, h, quickTask())
	exit, out := h.run("", "check")
	if exit != 0 {
		t.Fatalf("check: %d\n%s", exit, out)
	}
	for _, want := range []string{"Running the task's check: git --version", "git version", "passed", "is recorded for your session"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	end()
	evs := checkEvents(h)
	if len(evs) != 1 {
		t.Fatalf("%d check.run events: %+v", len(evs), evs)
	}
	ev := evs[0]
	if ev.Command != "git --version" || !ev.Passed || len(ev.Failing) != 0 || !strings.Contains(ev.OutputTail, "git version") || ev.DurationMS < 0 {
		t.Fatalf("event: %+v", ev)
	}
}

func TestCheckFailureExitsNonZeroAndIsRecorded(t *testing.T) {
	h := newHarness(t)
	task := fakeapi.DemoTask()
	task.CheckCommands = []string{"git --version", "git --no-such-option-for-the-test"}
	end := watching(t, h, task)
	exit, out := h.run("", "check")
	if exit == 0 {
		t.Fatalf("a failing check exited 0:\n%s", out)
	}
	for _, want := range []string{"passed", "failed (exit code"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	end()
	evs := checkEvents(h)
	if len(evs) != 2 || !evs[0].Passed || evs[1].Passed || evs[1].Command != "git --no-such-option-for-the-test" || evs[1].OutputTail == "" {
		t.Fatalf("events: %+v", evs)
	}
}

func TestCheckWithoutTheWatcherSaysItWasNotRecorded(t *testing.T) {
	h := newHarness(t)
	task := fakeapi.DemoTask()
	task.CheckCommands = []string{"git --version"}
	h.started(fakeapi.SessionOptions{Task: task})
	exit, out := h.run("", "check")
	if exit != 0 || !strings.Contains(out, "Not recorded") || !strings.Contains(out, "passed") {
		t.Fatalf("check with no watcher: %d\n%s", exit, out)
	}
	task.CheckCommands = []string{"git --no-such-option-for-the-test"}
	h2 := newHarness(t)
	h2.started(fakeapi.SessionOptions{Task: task})
	if exit, out := h2.run("", "check"); exit != 1 || !strings.Contains(out, "Not recorded") {
		t.Fatalf("failing check with no watcher: %d\n%s", exit, out)
	}
	if n := len(checkEvents(h)) + len(checkEvents(h2)); n != 0 {
		t.Fatalf("%d events recorded with no watcher", n)
	}
}

func TestCheckNeedsASession(t *testing.T) {
	h := newHarness(t)
	if exit, out := h.run("", "check"); exit == 0 || !strings.Contains(out, "no active assessment") {
		t.Fatalf("check with no session: %d\n%s", exit, out)
	}
}

func TestSubmitRecordsTheCheckRun(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask()})
	writeFile(t, dir, "notes.md", "Design notes\n")
	if exit, out := h.run("", "submit", "-m", "done", "--yes"); exit != 0 {
		t.Fatalf("submit: %d\n%s", exit, out)
	}
	evs := checkEvents(h)
	if len(evs) != 1 || evs[0].Command != "git --version" || !evs[0].Passed {
		t.Fatalf("submit's check events: %+v", evs)
	}
}
