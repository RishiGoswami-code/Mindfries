package cli

import (
	"bytes"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/sys"
	"github.com/mindfries/v2/cli/internal/workspace"
)

// harness is a fake API, a fake machine and a temporary MINDFRIES_HOME.
type harness struct {
	t       *testing.T
	srv     *fakeapi.Server
	url     string
	home    string
	machine *sys.Fake
	env     map[string]string

	mu     sync.Mutex
	offset time.Duration
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	workspace.Backoff = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	h := &harness{t: t, home: t.TempDir()}
	h.srv = fakeapi.New(fakeapi.Config{Now: h.now})
	ts := httptest.NewServer(h.srv.Handler())
	t.Cleanup(ts.Close)
	h.url = ts.URL + fakeapi.Prefix
	h.machine = &sys.Fake{
		Programs: map[string]string{"git": "/usr/bin/git", "python3": "/usr/bin/python3"},
		Outputs:  map[string]string{"git --version": "git version 2.43.0", "python3 --version": "Python 3.12.1"},
		Env:      map[string]string{"HOME": t.TempDir()},
		Free:     10 << 30,
	}
	h.env = map[string]string{
		"MINDFRIES_HOME":        h.home,
		"MINDFRIES_API":         h.url,
		"MINDFRIES_TOKEN_STORE": "file",
	}
	return h
}

func (h *harness) now() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return time.Now().Add(h.offset)
}

// advance moves the fake API's clock.
func (h *harness) advance(d time.Duration) {
	h.mu.Lock()
	h.offset += d
	h.mu.Unlock()
}

func (h *harness) app(stdin string, out *bytes.Buffer) *App {
	return &App{
		Stdin:  strings.NewReader(stdin),
		Stdout: out,
		Stderr: out,
		Getenv: func(k string) string { return h.env[k] },
		Sys:    h.machine.Sys("linux", "amd64"),
	}
}

// run executes the CLI and returns its exit code and output.
func (h *harness) run(stdin string, args ...string) (int, string) {
	h.t.Helper()
	var out bytes.Buffer
	code := h.app(stdin, &out).Run(args)
	return code, out.String()
}

// started mints a code and runs start into a fresh folder, failing the
// test if it doesn't succeed. It returns the workspace folder.
func (h *harness) started(o fakeapi.SessionOptions, extra ...string) string {
	h.t.Helper()
	dir := filepath.Join(h.t.TempDir(), "ws")
	code := h.srv.MintSetupCode(o)
	args := append([]string{"start", code, "--dir", dir, "--yes"}, extra...)
	if exit, out := h.run("", args...); exit != 0 {
		h.t.Fatalf("start: %d\n%s", exit, out)
	}
	return dir
}
