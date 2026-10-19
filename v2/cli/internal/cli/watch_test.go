package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/state"
)

func TestStartRunsTheWatcherUntilTheSessionEnds(t *testing.T) {
	h := newHarness(t)
	h.keepWatching = true
	dir := wsDir(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{})
	done := make(chan string, 1)
	go func() {
		_, out := h.run("", "start", code, "--dir", dir, "--yes")
		done <- out
	}()
	var id string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if id = h.srv.OnlySessionID(); id != "" && len(h.srv.Events(id)) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	os.WriteFile(filepath.Join(dir, "limiter.py"), []byte("def allow(key):\n    return True\n"), 0o644)

	// status, from another terminal.
	for time.Now().Before(deadline) {
		_, out := h.run("", "status")
		if strings.Contains(out, "Watcher      running") && strings.Contains(out, "last synced") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	_, out := h.run("", "status")
	for _, want := range []string{"Watcher      running", "Workspace    " + dir, "branch mindfries/assessment"} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}

	h.srv.EndSession(id)
	select {
	case out := <-done:
		if !strings.Contains(out, "was ended") {
			t.Fatalf("start output:\n%s", out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("start didn't return after the session ended")
	}
	types := map[string]bool{}
	for _, e := range h.srv.Events(id) {
		types[e.Type] = true
	}
	for _, want := range []string{"preflight.result", "workspace.ready", "ide.launched", "file.changed"} {
		if !types[want] {
			t.Errorf("no %s event; got %v", want, types)
		}
	}
	if _, err := state.Load(h.home); err != state.ErrNone {
		t.Error("the local session survived the session ending")
	}
}
