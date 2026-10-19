package cli

import (
	"bytes"
	"context"
	"fmt"
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

func TestAgentOpenAtStart(t *testing.T) {
	for _, tc := range []struct {
		name, stdin string
		closed      bool
	}{
		{"started anyway", "n\n", false},
		{"closed it", "y\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.machine.SetProcs("aider")
			app := h.app(tc.stdin+wsDir(t)+"\n", &bytes.Buffer{})
			var out bytes.Buffer
			app.Stdout, app.Stderr = &out, &out
			if tc.closed {
				orig := app.Sys.Processes
				calls := 0
				app.Sys.Processes = func(ctx context.Context) ([]string, error) {
					calls++
					if calls > 1 {
						return nil, nil
					}
					return orig(ctx)
				}
			}
			if code := app.Run([]string{"start", h.srv.MintSetupCode(fakeapi.SessionOptions{})}); code != 0 {
				t.Fatalf("start: %d\n%s", code, out.String())
			}
			if !strings.Contains(out.String(), "Before you start") || !strings.Contains(out.String(), "These coding agents are running: aider") {
				t.Fatalf("output:\n%s", out.String())
			}
			var found bool
			for _, e := range h.srv.Events(h.srv.OnlySessionID()) {
				if e.Type == "agent_signal.process" && strings.Contains(string(e.Data), `"phase":"start"`) {
					found = true
					if want := fmt.Sprintf(`"closed_by_candidate":%v`, tc.closed); !strings.Contains(string(e.Data), want) {
						t.Errorf("event %s lacks %s", e.Data, want)
					}
				}
			}
			if !found {
				t.Fatal("no start-phase process signal")
			}
		})
	}
}
