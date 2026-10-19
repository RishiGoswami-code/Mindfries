package watcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/workspace"
)

const taskMD = "# The task\n"

// env is a fake API session plus a real Git workspace.
type env struct {
	t        *testing.T
	srv      *fakeapi.Server
	client   *api.Client
	local    *state.Local
	stateDir string
	ws       string
	q        *Queue
	out      syncBuf
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func newEnv(t *testing.T, o fakeapi.SessionOptions) *env {
	t.Helper()
	e := &env{t: t, srv: fakeapi.New(fakeapi.Config{}), stateDir: t.TempDir(), ws: t.TempDir()}
	ts := httptest.NewServer(e.srv.Handler())
	t.Cleanup(ts.Close)
	e.client = api.New(ts.URL+fakeapi.Prefix, "0.1.0")
	ctx := context.Background()
	res, err := e.client.Exchange(ctx, api.ExchangeRequest{SetupCode: e.srv.MintSetupCode(o), Device: api.Device{OS: "linux", Arch: "amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	e.client.Token = res.SessionToken

	r := &gitx.Repo{Dir: e.ws}
	os.WriteFile(filepath.Join(e.ws, "TASK.md"), []byte(taskMD), 0o644)
	os.WriteFile(filepath.Join(e.ws, "main.py"), []byte("print('hi')\n"), 0o644)
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "C"}, {"config", "user.email", "c@x"}, {"config", "commit.gpgsign", "false"}, {"add", "-A"}, {"commit", "-q", "-m", "base"}} {
		if _, err := r.Run(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	base, _ := r.Head(ctx)
	sum := workspace.HashBytes([]byte(taskMD))
	state.WriteFileAtomic(filepath.Join(e.stateDir, "locked", sum), []byte(taskMD), 0o600)
	os.Chmod(filepath.Join(e.ws, "TASK.md"), 0o444)
	e.local = &state.Local{
		SessionID: res.Session.ID, EndsAt: res.Session.EndsAt, Intervals: res.Session.Intervals, Limits: res.Session.Limits,
		Workspace: e.ws, BaseCommit: base, Locked: []state.LockedFile{{Path: "TASK.md", SHA256: sum}},
	}
	e.q, err = OpenQueue(e.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) config() Config {
	return Config{
		Client: e.client, StateDir: e.stateDir, Local: e.local, Queue: e.q, Out: &e.out,
		Scan: 20 * time.Millisecond, Send: 30 * time.Millisecond, Heartbeat: 60 * time.Millisecond,
		Snapshot: 100 * time.Millisecond, AgentScan: 100 * time.Millisecond, MaxBackoff: 60 * time.Millisecond,
		FlushTimeout: 3 * time.Second,
	}
}

// run starts the watcher and returns a function that waits for it.
func (e *env) run(cfg Config) func() string {
	done := make(chan string, 1)
	go func() {
		r, err := New(cfg).Run(context.Background())
		if err != nil {
			e.t.Errorf("Run: %v", err)
		}
		done <- r
	}()
	// Wait for the first look at the workspace, so later writes are changes.
	e.eventually("watcher running", func() bool { st, err := ReadStatus(e.stateDir); return err == nil && st.PID != 0 })
	return func() string {
		select {
		case r := <-done:
			return r
		case <-time.After(15 * time.Second):
			e.t.Fatal("watcher didn't stop")
			return ""
		}
	}
}

func (e *env) eventually(what string, cond func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.t.Fatalf("timed out waiting for %s\n%s", what, e.out.String())
}

func (e *env) write(name, body string) {
	if err := os.WriteFile(filepath.Join(e.ws, name), []byte(body), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) serverEvents() []api.Event { return e.srv.Events(e.local.SessionID) }

func (e *env) ofType(typ string) []api.Event {
	var out []api.Event
	for _, ev := range e.serverEvents() {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

// checkExactlyOnce asserts the server holds events 1..last once each, with
// an intact chain.
func (e *env) checkExactlyOnce() {
	e.t.Helper()
	evs := e.serverEvents()
	last := e.q.LastSeq()
	if int64(len(evs)) != last {
		e.t.Fatalf("server has %d events, queue made %d", len(evs), last)
	}
	for i, ev := range evs {
		if ev.Seq != int64(i+1) {
			e.t.Fatalf("event %d has seq %d", i+1, ev.Seq)
		}
	}
	if n := e.srv.Notes(e.local.SessionID); len(n) > 0 {
		e.t.Fatalf("integrity notes: %v", n)
	}
}

func TestNetworkCutLosesAndDuplicatesNothing(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{Limits: api.Limits{MaxEventsPerBatch: 7, MaxBatchBytes: 1 << 20, MaxSnapshotBytes: 2 << 20, MaxBundleBytes: 1 << 20}})
	wait := e.run(e.config())
	e.write("a.py", "x = 1\n")
	e.eventually("first event delivered", func() bool { return len(e.ofType("file.changed")) == 1 })

	// The network goes. The scaled-down version of a ten-minute outage:
	// the sender backs off and retries many times while work goes on.
	e.srv.SetOffline(true)
	for i := 0; i < 40; i++ {
		e.write(fmt.Sprintf("offline_%02d.py", i), fmt.Sprintf("v = %d\n", i))
		time.Sleep(25 * time.Millisecond)
	}
	e.eventually("40 offline changes queued", func() bool { return e.q.Depth() >= 40 })
	if !strings.Contains(e.out.String(), "Can't reach Mindfries") {
		t.Errorf("no offline notice:\n%s", e.out.String())
	}
	st, _ := ReadStatus(e.stateDir)
	if st == nil || st.OfflineSince == nil {
		t.Errorf("status doesn't show offline: %+v", st)
	}

	// It comes back, and the first two replies are lost after the server
	// stored the batch, so the CLI must resend what it already sent.
	e.srv.DropReplies(2)
	e.srv.SetOffline(false)
	e.eventually("queue drained", func() bool { return e.q.Depth() == 0 && len(e.ofType("file.changed")) == 41 })
	if !strings.Contains(e.out.String(), "Back online") {
		t.Errorf("no back-online notice:\n%s", e.out.String())
	}
	RequestStop(e.stateDir, ReasonStop)
	if r := wait(); r != ReasonStop {
		t.Fatalf("reason = %q", r)
	}
	e.checkExactlyOnce()
	if got := e.ofType("watcher.stopped"); len(got) != 1 {
		t.Fatalf("watcher.stopped events: %d", len(got))
	}
}

func TestLockedFileIsRestored(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	wait := e.run(e.config())
	p := filepath.Join(e.ws, "TASK.md")
	os.Chmod(p, 0o644)
	os.WriteFile(p, []byte("# An easier task\n"), 0o644)
	e.eventually("restored", func() bool {
		b, _ := os.ReadFile(p)
		return string(b) == taskMD && len(e.ofType("task_file.modified")) == 1
	})
	var d struct {
		Path     string `json:"path"`
		Restored bool   `json:"restored"`
	}
	json.Unmarshal(e.ofType("task_file.modified")[0].Data, &d)
	if d.Path != "TASK.md" || !d.Restored {
		t.Fatalf("event: %+v", d)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm()&0o222 != 0 {
		t.Fatal("restored TASK.md is writable")
	}
	if len(e.ofType("file.changed")) != 0 {
		t.Fatal("a locked file's edit was also reported as an ordinary change")
	}
	RequestStop(e.stateDir, ReasonStop)
	wait()
}

func TestRefusedBatchIsDropped(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	e.srv.RefuseBatches(1)
	wait := e.run(e.config())
	e.write("one.py", "1\n")
	e.eventually("refusal", func() bool { return strings.Contains(e.out.String(), "dropped") })
	e.write("two.py", "2\n")
	e.eventually("later event delivered", func() bool { return len(e.ofType("file.changed")) >= 1 })
	RequestStop(e.stateDir, ReasonStop)
	wait()
	if e.q.Depth() != 0 {
		t.Fatalf("queue still holds %d", e.q.Depth())
	}
}

func TestSnapshotsAreSent(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	wait := e.run(e.config())
	e.write("main.py", "print('changed')\n")
	e.write("new.py", "y = 2\n")
	e.eventually("snapshot", func() bool {
		for _, p := range e.srv.Snapshots(e.local.SessionID) {
			if strings.Contains(p, "print('changed')") && strings.Contains(p, "new.py") {
				return true
			}
		}
		return false
	})
	// The candidate's own index is untouched.
	out, _ := (&gitx.Repo{Dir: e.ws}).Run(context.Background(), "diff", "--cached", "--name-only")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("snapshot staged files: %q", out)
	}
	RequestStop(e.stateDir, ReasonStop)
	wait()
}

func TestSessionEndedFromPortal(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	wait := e.run(e.config())
	e.eventually("heartbeat", func() bool { return e.srv.Heartbeats(e.local.SessionID) > 0 })
	e.srv.EndSession(e.local.SessionID)
	if r := wait(); r != ReasonRevoked {
		t.Fatalf("reason = %q", r)
	}
}

func TestExpiry(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{Duration: 400 * time.Millisecond})
	if r := e.run(e.config())(); r != ReasonExpired {
		t.Fatalf("reason = %q", r)
	}
}

func TestGapAfterCrash(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	// A watcher that died a minute ago left its status behind.
	writeStatus(e.stateDir, &Status{UpdatedAt: time.Now().Add(-time.Minute)})
	os.WriteFile(lockPath(e.stateDir), []byte("1"), 0o600)
	wait := e.run(e.config())
	e.eventually("gap", func() bool { return len(e.ofType("watcher.gap")) == 1 })
	var d struct{ Reason string }
	json.Unmarshal(e.ofType("watcher.gap")[0].Data, &d)
	if d.Reason != "crash" {
		t.Fatalf("gap reason %q", d.Reason)
	}
	RequestStop(e.stateDir, ReasonStop)
	wait()
}

func TestSecondWatcherRefused(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	wait := e.run(e.config())
	if _, err := New(e.config()).Run(context.Background()); err != ErrAlreadyRunning {
		t.Fatalf("second watcher: %v", err)
	}
	RequestStop(e.stateDir, ReasonStop)
	wait()
	if Running(e.stateDir) {
		t.Fatal("still marked running")
	}
}

func TestQueueSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	q, _ := OpenQueue(dir)
	a, _ := q.Append("x", map[string]int{"n": 1})
	b, _ := q.Append("x", map[string]int{"n": 2})
	if b.PrevHash != a.Hash {
		t.Fatal("not chained")
	}
	q.Ack(1)
	q2, err := OpenQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	if q2.LastSeq() != 2 || q2.Acked() != 1 || q2.Depth() != 1 {
		t.Fatalf("reopened: %s", q2)
	}
	c, _ := q2.Append("x", nil)
	if c.Seq != 3 || c.PrevHash != b.Hash {
		t.Fatalf("chain after reopen: %+v", c)
	}
	q2.Ack(3)
	q3, _ := OpenQueue(dir)
	d, _ := q3.Append("x", nil)
	if d.Seq != 4 || d.PrevHash != c.Hash {
		t.Fatalf("chain after full compaction: %+v", d)
	}
}

func readFile(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, name))
	return string(b), err
}
