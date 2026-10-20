package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/api"
)

// The monitoring panel: start doesn't begin until camera, screen and
// microphone are on, and stills and audio clips are kept once it has.
func TestStartWaitsForMonitoringThenBegins(t *testing.T) {
	s := newSession(t)
	wait := s.startWith(fakeapi.SessionOptions{Monitoring: &api.Monitoring{Camera: true, Screen: true, Audio: true}})
	id := s.srv.OnlySessionID()
	if b := s.srv.Begun(id); b == nil || b.IDE != "manual" || len(b.Checks) == 0 {
		t.Fatalf("begun with %+v", b)
	}
	if st := s.run("status"); !strings.Contains(st, "Monitoring   on") {
		t.Fatalf("status:\n%s", st)
	}
	// The panel sends a still and an audio clip; both are kept.
	sess, _ := s.srv.SessionInfo(id)
	panel := sess.Monitoring.PanelURL
	for _, u := range []string{panel + "/still?kind=camera", panel + "/still?kind=screen", panel + "/audio"} {
		resp, err := http.Post(u, "application/octet-stream", strings.NewReader("data"))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if stills, clips := s.srv.Captures(id); stills != 2 || clips != 1 {
		t.Fatalf("captures: %d stills, %d clips", stills, clips)
	}
	s.run("stop", "--yes")
	wait()
}

// setup without a code signs in through the browser, and sets everything up
// without starting the clock.
func TestSetupWithBrowserSignIn(t *testing.T) {
	s := newSession(t)
	inv := s.srv.Invite(fakeapi.SessionOptions{Title: "E2E task", Task: task()})
	c := s.cmd("setup", "--dir", s.ws, "--yes")
	var out syncBuf
	c.Stdout, c.Stderr = &out, &out
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	s.confirmInBrowser(&out, inv)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("setup: %v\n%s", err, out.String())
		}
	case <-time.After(60 * time.Second):
		_ = c.Process.Kill()
		t.Fatalf("setup didn't finish:\n%s", out.String())
	}
	sess, _ := s.srv.SessionInfo(s.srv.OnlySessionID())
	if sess.Status != api.StatusReady || !strings.Contains(out.String(), "Ready:") {
		t.Fatalf("status %s\n%s", sess.Status, out.String())
	}
	s.run("stop", "--yes")
}
