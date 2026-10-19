package cli

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/sys"
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
	h := &harness{t: t, home: t.TempDir()}
	h.srv = fakeapi.New(fakeapi.Config{Now: h.now})
	ts := httptest.NewServer(h.srv.Handler())
	t.Cleanup(ts.Close)
	h.url = ts.URL + fakeapi.Prefix
	h.machine = &sys.Fake{
		Programs: map[string]string{"git": "/usr/bin/git"},
		Outputs:  map[string]string{"git --version": "git version 2.43.0"},
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
