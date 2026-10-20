package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
)

// confirmInBrowser waits for start to ask for a sign-in, then does what
// the candidate does on the page: approve (picking invitationID) or refuse.
func (h *harness) confirmInBrowser(invitationID string, approve bool) <-chan error {
	done := make(chan error, 1)
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if codes := h.srv.PendingSignIns(); len(codes) > 0 {
				if approve {
					done <- h.srv.ApproveSignIn(codes[0], invitationID, "candidate@example.com")
				} else {
					done <- h.srv.DenySignIn(codes[0])
				}
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		done <- nil
	}()
	return done
}

func TestStartSignsInThroughTheBrowser(t *testing.T) {
	h := newHarness(t)
	h.machine.Env["DISPLAY"] = ":0"
	h.machine.Programs["xdg-open"] = "/usr/bin/xdg-open"
	h.srv.Invite(fakeapi.SessionOptions{Title: "Not this one"})
	want := h.srv.Invite(fakeapi.SessionOptions{Title: "Rate limiter", Task: quickTask()})
	confirmed := h.confirmInBrowser(want, true)

	dir := filepath.Join(t.TempDir(), "ws")
	exit, out := h.run("", "setup", "--dir", dir, "--yes")
	if err := <-confirmed; err != nil {
		t.Fatal(err)
	}
	if exit != 0 {
		t.Fatalf("start: %d\n%s", exit, out)
	}
	for _, w := range []string{"Sign in to connect this computer", "Check the page shows this code: ", "Signed in.", "Connected to Rate limiter"} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
	opened := false
	for _, s := range h.machine.Started {
		if strings.HasPrefix(s, "xdg-open http") && strings.Contains(s, "/device?code=") {
			opened = true
		}
	}
	if !opened {
		t.Errorf("browser not opened: %v", h.machine.Started)
	}
	if exit, out := h.run("", "status"); exit != 0 || !strings.Contains(out, "Rate limiter") {
		t.Fatalf("status after sign-in: %d\n%s", exit, out)
	}
}

func TestStartSignInWithoutABrowser(t *testing.T) {
	h := newHarness(t)
	h.srv.Invite(fakeapi.SessionOptions{Task: quickTask()})
	confirmed := h.confirmInBrowser("", true)
	// No DISPLAY: nothing to open, so the address is printed instead.
	exit, out := h.run("", "setup", "--dir", filepath.Join(t.TempDir(), "ws"), "--yes")
	<-confirmed
	if exit != 0 || !strings.Contains(out, "Open this address in your browser:") {
		t.Fatalf("start: %d\n%s", exit, out)
	}
	if len(h.machine.Started) > 0 && strings.Contains(strings.Join(h.machine.Started, "\n"), "/device") {
		t.Fatalf("tried to open a browser: %v", h.machine.Started)
	}
}

func TestStartSignInRefused(t *testing.T) {
	h := newHarness(t)
	h.srv.Invite(fakeapi.SessionOptions{})
	refused := h.confirmInBrowser("", false)
	exit, out := h.run("", "setup", "--yes", "--no-browser")
	<-refused
	if exit == 0 || !strings.Contains(out, "refused in the browser") {
		t.Fatalf("start: %d\n%s", exit, out)
	}
	if exit, out := h.run("", "status"); exit != 0 || !strings.Contains(out, "No active assessment") {
		t.Fatalf("status: %d\n%s", exit, out)
	}
}

func TestStartWaitsForASleepingServer(t *testing.T) {
	oldFor, oldEvery := wakeFor, wakeEvery
	wakeFor, wakeEvery = 5*time.Second, 10*time.Millisecond
	defer func() { wakeFor, wakeEvery = oldFor, oldEvery }()

	h := newHarness(t)
	h.srv.Invite(fakeapi.SessionOptions{Task: quickTask()})
	// The first few requests get 502, as Render answers while it wakes.
	fails := 3
	real := h.srv.Handler()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fails > 0 {
			fails--
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		real.ServeHTTP(w, r)
	}))
	defer ts.Close()
	h.env["MINDFRIES_API"] = ts.URL + fakeapi.Prefix
	confirmed := h.confirmInBrowser("", true)
	exit, out := h.run("", "setup", "--dir", filepath.Join(t.TempDir(), "ws"), "--yes", "--no-browser")
	<-confirmed
	if exit != 0 || !strings.Contains(out, "Still trying") || !strings.Contains(out, "Signed in.") {
		t.Fatalf("start: %d\n%s", exit, out)
	}
}
