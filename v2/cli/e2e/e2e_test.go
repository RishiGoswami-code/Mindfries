// Package e2e runs the real mindfries binary through whole sessions against
// the fake API: start, work, status, submit, and start, stop. CI runs it on
// Linux, macOS and Windows (v2/PLAN.md task 1.13).
package e2e

import (
	"bytes"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/api"
)

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// binary builds mindfries once for all tests.
func binary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mindfries-e2e-*")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "mindfries")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		out, err := exec.Command("go", "build", "-o", binPath, "../cmd/mindfries").CombinedOutput()
		if err != nil {
			buildErr = err
			binPath = string(out)
		}
	})
	if buildErr != nil {
		t.Fatalf("build: %v %s", buildErr, binPath)
	}
	return binPath
}

type session struct {
	t    *testing.T
	srv  *fakeapi.Server
	env  []string
	home string
	ws   string
}

func newSession(t *testing.T) *session {
	s := &session{t: t, srv: fakeapi.New(fakeapi.Config{}), home: t.TempDir(), ws: filepath.Join(t.TempDir(), "workspace")}
	ts := httptest.NewServer(s.srv.Handler())
	t.Cleanup(ts.Close)
	s.env = append(os.Environ(),
		"MINDFRIES_HOME="+s.home,
		"MINDFRIES_API="+ts.URL+fakeapi.Prefix,
		"MINDFRIES_TOKEN_STORE=file",
	)
	return s
}

func task() *fakeapi.Task {
	t := fakeapi.DemoTask()
	t.Runtimes = nil
	t.CheckCommands = []string{"git --version"}
	return t
}

func (s *session) cmd(args ...string) *exec.Cmd {
	c := exec.Command(binary(s.t), args...)
	c.Env = s.env
	c.Stdin = strings.NewReader("")
	return c
}

// run runs a command to completion.
func (s *session) run(args ...string) string {
	s.t.Helper()
	out, err := s.cmd(args...).CombinedOutput()
	if err != nil {
		s.t.Fatalf("mindfries %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuf) Write(p []byte) (int, error) { b.mu.Lock(); defer b.mu.Unlock(); return b.b.Write(p) }
func (b *syncBuf) String() string              { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

// start launches `mindfries start` (it runs the watcher until the session
// ends) and waits until the watcher is up.
func (s *session) start() (wait func() string) {
	s.t.Helper()
	code := s.srv.MintSetupCode(fakeapi.SessionOptions{
		Title:     "E2E task",
		Task:      task(),
		Intervals: api.Intervals{HeartbeatSeconds: 1, SnapshotSeconds: 2, AgentScanSeconds: 2},
	})
	c := s.cmd("start", code, "--dir", s.ws, "--ide", "manual", "--yes")
	var out syncBuf
	c.Stdout, c.Stderr = &out, &out
	if err := c.Start(); err != nil {
		s.t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	s.eventually("watcher running", &out, func() bool {
		return strings.Contains(s.run("status"), "Watcher      running")
	})
	return func() string {
		select {
		case err := <-done:
			if err != nil {
				s.t.Fatalf("start exited with %v\n%s", err, out.String())
			}
		case <-time.After(60 * time.Second):
			_ = c.Process.Kill()
			s.t.Fatalf("start didn't finish\n%s", out.String())
		}
		return out.String()
	}
}

func (s *session) eventually(what string, out *syncBuf, cond func() bool) {
	s.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for %s\n%s", what, out.String())
}

func (s *session) noCredentialsLeft() {
	s.t.Helper()
	if _, err := os.Stat(filepath.Join(s.home, "session")); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(filepath.Join(s.home, "session"))
		s.t.Fatalf("session state left on disk: %v", entries)
	}
}

func TestStartWorkSubmit(t *testing.T) {
	s := newSession(t)
	wait := s.start()
	id := s.srv.OnlySessionID()

	// Work like a candidate.
	write(t, s.ws, "limiter.py", "import time\n\n_hits = {}\n\n\ndef allow(key):\n    now = time.time()\n    hits = [h for h in _hits.get(key, []) if now - h < 60]\n    if len(hits) >= 5:\n        return False\n    _hits[key] = hits + [now]\n    return True\n")
	write(t, s.ws, "NOTES.md", "Sliding window per key.\n")
	var watched syncBuf
	s.eventually("changes recorded", &watched, func() bool {
		n := 0
		for _, e := range s.srv.Events(id) {
			if e.Type == "file.changed" {
				n++
			}
		}
		return n >= 2
	})

	status := s.run("status")
	for _, want := range []string{"E2E task", "active", "Workspace    " + s.ws, "branch mindfries/assessment"} {
		if !strings.Contains(status, want) {
			t.Errorf("status lacks %q:\n%s", want, status)
		}
	}

	out := s.run("submit", "-m", "Sliding-window rate limiter", "--yes")
	if !strings.Contains(out, "Submitted. Commit") {
		t.Fatalf("submit:\n%s", out)
	}
	startOut := wait()
	if !strings.Contains(startOut, "Workspace ready") {
		t.Errorf("start output:\n%s", startOut)
	}

	sub, bundle := s.srv.Submission(id)
	if sub == nil || sub.Status != api.SubmissionVerified || len(bundle) == 0 {
		t.Fatalf("submission: %+v", sub)
	}
	types := map[string]int{}
	for _, e := range s.srv.Events(id) {
		types[e.Type]++
	}
	for _, want := range []string{"workspace.ready", "ide.launched", "file.changed", "git.commit", "watcher.stopped"} {
		if types[want] == 0 {
			t.Errorf("no %s event: %v", want, types)
		}
	}
	if notes := s.srv.Notes(id); len(notes) > 0 {
		t.Errorf("event chain problems: %v", notes)
	}
	s.noCredentialsLeft()
	if !strings.Contains(s.run("status"), "Last submitted") {
		t.Error("status doesn't show the submission")
	}
	if b, err := os.ReadFile(filepath.Join(s.ws, "limiter.py")); err != nil || !strings.Contains(string(b), "_hits") {
		t.Error("the candidate's code isn't in the workspace any more")
	}
}

func TestStartStop(t *testing.T) {
	s := newSession(t)
	wait := s.start()
	id := s.srv.OnlySessionID()
	write(t, s.ws, "limiter.py", "def allow(key):\n    return True\n")
	out := s.run("stop", "--yes")
	if !strings.Contains(out, "The assessment has ended") {
		t.Fatalf("stop:\n%s", out)
	}
	wait()
	if !s.srv.Revoked(id) {
		t.Fatal("token not revoked")
	}
	s.noCredentialsLeft()
	if _, err := os.Stat(filepath.Join(s.ws, "limiter.py")); err != nil {
		t.Fatal("stop removed the candidate's code")
	}
	var stopped bool
	for _, e := range s.srv.Events(id) {
		stopped = stopped || (e.Type == "watcher.stopped" && strings.Contains(string(e.Data), `"stop"`))
	}
	if !stopped {
		t.Error("no watcher.stopped event")
	}
}

func TestVersionAndDoctor(t *testing.T) {
	s := newSession(t)
	if out := s.run("--version"); !strings.HasPrefix(out, "mindfries ") {
		t.Fatalf("--version: %q", out)
	}
	out, _ := s.cmd("doctor").CombinedOutput()
	if !strings.Contains(string(out), "Git") || !strings.Contains(string(out), "Mindfries API") {
		t.Fatalf("doctor:\n%s", out)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
