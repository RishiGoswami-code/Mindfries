package cli

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/state"
)

func TestStartExchangesCodeAndStoresToken(t *testing.T) {
	h := newHarness(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Title: "Backend task"})
	exit, out := h.run("", "start", strings.ToLower(code))
	if exit != 0 {
		t.Fatalf("start: %d\n%s", exit, out)
	}
	if !strings.Contains(out, "Connected to Backend task") {
		t.Fatalf("output:\n%s", out)
	}
	l, err := state.Load(h.home)
	if err != nil {
		t.Fatal(err)
	}
	if l.TokenStore != "file" || l.SessionID == "" {
		t.Fatalf("state: %+v", l)
	}
	tok, err := os.ReadFile(filepath.Join(state.Dir(h.home), "token"))
	if err != nil {
		t.Fatal(err)
	}
	secretPart := strings.TrimSpace(strings.SplitN(string(tok), "\n", 2)[1])
	if secretPart == "" || strings.Contains(out, secretPart) {
		t.Fatal("the token was printed")
	}
	session, _ := os.ReadFile(filepath.Join(state.Dir(h.home), "session.json"))
	if strings.Contains(string(session), secretPart) {
		t.Fatal("the token is in session.json")
	}

	exit, out = h.run("", "status")
	if exit != 0 || !strings.Contains(out, "active,") {
		t.Fatalf("status: %d\n%s", exit, out)
	}
}

func TestReusedCodeIsRefused(t *testing.T) {
	h := newHarness(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{})
	if exit, out := h.run("", "start", code); exit != 0 {
		t.Fatalf("first use: %s", out)
	}
	// A second computer (fresh home) tries the same code.
	h.home = t.TempDir()
	h.env["MINDFRIES_HOME"] = h.home
	exit, out := h.run("", "start", code)
	if exit == 0 || !strings.Contains(out, "already been used") {
		t.Fatalf("reuse: %d\n%s", exit, out)
	}
}

func TestExpiredCodeIsRefused(t *testing.T) {
	h := newHarness(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{CodeTTL: 10 * time.Minute})
	h.advance(11 * time.Minute)
	exit, out := h.run("", "start", code)
	if exit == 0 || !strings.Contains(out, "expired") {
		t.Fatalf("expired code: %d\n%s", exit, out)
	}
	if _, err := state.Load(h.home); err != state.ErrNone {
		t.Fatal("an expired code left a session behind")
	}
}

func TestUnknownAndMalformedCodes(t *testing.T) {
	h := newHarness(t)
	if exit, out := h.run("", "start", "AAAA-BBBB-CCCC"); exit == 0 || !strings.Contains(out, "isn't recognised") {
		t.Fatalf("unknown: %s", out)
	}
	if exit, out := h.run("", "start", "hello"); exit == 0 || !strings.Contains(out, "doesn't look like a setup code") {
		t.Fatalf("malformed: %s", out)
	}
	// Without a code, start signs in instead (signin_test.go).
}

func TestExpiredTokenIsRefused(t *testing.T) {
	h := newHarness(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Duration: time.Hour})
	if exit, out := h.run("", "start", code); exit != 0 {
		t.Fatalf("start: %s", out)
	}
	h.advance(61 * time.Minute)
	exit, out := h.run("", "start")
	if exit == 0 || !strings.Contains(out, "run out of time") {
		t.Fatalf("expired token: %d\n%s", exit, out)
	}
	if _, err := os.Stat(filepath.Join(state.Dir(h.home), "token")); !os.IsNotExist(err) {
		t.Fatal("token left on disk after the session expired")
	}
}

func TestRevokedTokenIsRefused(t *testing.T) {
	h := newHarness(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{})
	h.run("", "start", code)
	_, c, err := h.app("", nil).active()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Revoke(t.Context(), "stop"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Session(t.Context()); err == nil {
		t.Fatal("a revoked token still works")
	}
}

func TestOldCLIIsRefused(t *testing.T) {
	h := newHarness(t)
	old := fakeapi.New(fakeapi.Config{MinVersion: "9.0.0"})
	ts := httptest.NewServer(old.Handler())
	defer ts.Close()
	h.env["MINDFRIES_API"] = ts.URL + fakeapi.Prefix
	exit, out := h.run("", "start", old.MintSetupCode(fakeapi.SessionOptions{}))
	if exit == 0 || !strings.Contains(out, "too old (need 9.0.0") || !strings.Contains(out, "Run the Mindfries install command again") {
		t.Fatalf("%d\n%s", exit, out)
	}
}
