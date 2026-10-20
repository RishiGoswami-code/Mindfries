package httpapi

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/session"
)

// These tests run the real `mindfries` binary against this server, on a real
// database: the CLI's own end-to-end flow (v2/cli/e2e), now against the real
// API instead of the fake one. They need V2_TEST_DATABASE_URL, git and go.

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// cliBinary builds mindfries once for all tests.
func cliBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode")
	}
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mindfries-real-e2e-*")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "mindfries")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		wd, _ := os.Getwd() // .../v2/api/internal/httpapi
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/mindfries")
		cmd.Dir = filepath.Clean(filepath.Join(wd, "../../../cli"))
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildError{err, string(out)}
		}
	})
	if buildErr != nil {
		t.Fatalf("building the CLI: %v", buildErr)
	}
	return binPath
}

type buildError struct {
	err error
	out string
}

func (b *buildError) Error() string { return b.err.Error() + "\n" + b.out }

// cliRun is one candidate's computer.
type cliRun struct {
	t    *testing.T
	e    *env
	home string
	ws   string
	env  []string
}

func newCLIRun(t *testing.T, e *env) *cliRun {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git isn't installed")
	}
	c := &cliRun{t: t, e: e, home: t.TempDir(), ws: filepath.Join(t.TempDir(), "workspace")}
	c.env = append(os.Environ(),
		"MINDFRIES_HOME="+c.home,
		"MINDFRIES_API="+e.ts.URL+cliPrefix,
		"MINDFRIES_TOKEN_STORE=file",
		"MINDFRIES_NO_BROWSER=1",
	)
	return c
}

func (c *cliRun) cmd(args ...string) *exec.Cmd {
	cmd := exec.Command(cliBinary(c.t), args...)
	cmd.Env = c.env
	cmd.Stdin = strings.NewReader("")
	return cmd
}

func (c *cliRun) run(args ...string) string {
	c.t.Helper()
	out, err := c.cmd(args...).CombinedOutput()
	if err != nil {
		c.t.Fatalf("mindfries %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func waitFor(t *testing.T, what string, out *lockedBuf, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s\n%s", what, out.String())
}

var setupCmd = regexp.MustCompile(`mindfries setup [A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}`)

func TestTheRealCLIGoesFromSetupToSubmitOnTheRealAPI(t *testing.T) {
	e := newEnv(t, nil)
	c := newCLIRun(t, e)
	g := newGrant()

	// The candidate presses Start assessment: the portal's server mints the command.
	m := e.portal("POST", "/v2/portal/setup-codes", g)
	if m.Status != 201 || !setupCmd.MatchString(m.str("cli_command")) {
		t.Fatalf("%d %s", m.Status, m.Raw)
	}

	// `mindfries setup` downloads the task and the codebase.
	out := c.run("setup", m.str("setup_code"), "--dir", c.ws, "--yes")
	if !strings.Contains(out, "Ready:") || !strings.Contains(out, "mindfries start") {
		t.Fatalf("setup:\n%s", out)
	}
	if st := c.run("status"); !strings.Contains(st, "set up, not started") {
		t.Fatalf("status after setup:\n%s", st)
	}
	live, ok, _ := e.db.LiveSessionFor(t.Context(), g["candidate_id"].(string), g["assessment_id"].(string))
	if !ok || live.Status != "ready" {
		t.Fatalf("the server has a ready session: %+v %v", live, ok)
	}
	sid := live.ID
	if _, err := os.Stat(filepath.Join(c.ws, "TASK.md")); err != nil {
		t.Fatal("the workspace has the task brief")
	}

	// `mindfries start`: environment checks, then it waits for the monitoring panel.
	start := c.cmd("start", "--ide", "manual", "--yes")
	var startOut lockedBuf
	start.Stdout, start.Stderr = &startOut, &startOut
	if err := start.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- start.Wait() }()
	t.Cleanup(func() { _ = start.Process.Kill() })

	panelURL := "https://portal.test/v2/session/" + sid
	waitFor(t, "start to ask for the monitoring panel", &startOut, func() bool { return strings.Contains(startOut.String(), panelURL) })
	if s, _ := e.db.SessionByID(t.Context(), sid); s.Status != "ready" {
		t.Fatalf("the clock must not run before the panel is live: %s", s.Status)
	}

	// The candidate's browser: the portal page gets a panel token and shares everything.
	tok := e.panelToken(g, sid)
	e.shareAll(tok)
	waitFor(t, "the clock to start", &startOut, func() bool {
		s, _ := e.db.SessionByID(t.Context(), sid)
		return s.Status == "active"
	})
	// The panel keeps reporting, as the page does every few seconds.
	stop := make(chan struct{})
	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				e.shareAll(tok)
			}
		}
	}()
	defer close(stop)

	waitFor(t, "the watcher", &startOut, func() bool { return strings.Contains(c.run("status"), "Watcher      running") })

	// Work like a candidate.
	if err := os.WriteFile(filepath.Join(c.ws, "NOTES.md"), []byte("Sliding window per key.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the changes to reach the server", &startOut, func() bool {
		evs, _ := e.db.EventsAfter(t.Context(), sid, 0, 500)
		n := 0
		for _, ev := range evs {
			if ev.Type == "file.changed" {
				n++
			}
		}
		return n >= 1
	})

	// The tracker has what happened, in plain words.
	feed := e.panel("GET", "/v2/panel/activity", tok, nil).JSON()["items"].([]any)
	var lines []string
	for _, it := range feed {
		lines = append(lines, it.(map[string]any)["text"].(string))
	}
	all := strings.Join(lines, "\n")
	for _, want := range []string{"Assessment started", "Working in your own editor", "Computer checks passed"} {
		if !strings.Contains(all, want) {
			t.Errorf("the tracker lacks %q:\n%s", want, all)
		}
	}

	if !strings.Contains(all, "workspace was reopened") && !strings.Contains(all, "set up on your computer") {
		t.Errorf("the tracker should say the workspace is ready: %s", all)
	}

	// Submit.
	sub := c.run("submit", "-m", "Sliding-window rate limiter", "--yes")
	if !strings.Contains(sub, "Submitted. Commit") {
		t.Fatalf("submit:\n%s", sub)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("start exited with %v\n%s", err, startOut.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatalf("start didn't finish\n%s", startOut.String())
	}
	if !strings.Contains(startOut.String(), "Started. Your time ends at") {
		t.Errorf("start output:\n%s", startOut.String())
	}

	// What the server holds.
	final, _ := e.db.SessionByID(t.Context(), sid)
	if final.Status != "submitted" {
		t.Fatalf("status %s", final.Status)
	}
	subm, err := e.db.SubmissionBySession(t.Context(), sid)
	if err != nil || subm.Status != "verified" {
		t.Fatalf("submission: %+v %v", subm, err)
	}
	evs, _ := e.db.EventsAfter(t.Context(), sid, 0, 5000)
	types := map[string]int{}
	for _, ev := range evs {
		types[ev.Type]++
	}
	for _, want := range []string{"workspace.ready", "file.changed", "git.commit", "watcher.stopped"} {
		if types[want] == 0 {
			t.Errorf("no %s event stored: %v", want, types)
		}
	}
	if left, err := os.ReadDir(filepath.Join(c.home, "session")); err == nil && len(left) > 0 {
		t.Errorf("session state left on disk: %v", left)
	}
	if b, err := io.ReadAll(strings.NewReader(c.run("status"))); err != nil || !strings.Contains(string(b), "Last submitted") {
		t.Errorf("status doesn't show the submission:\n%s", b)
	}
	_ = session.Session{}
}
