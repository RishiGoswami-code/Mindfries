package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/secret"
	"github.com/mindfries/v2/cli/internal/state"
)

// useKeychain stores tokens in an in-memory "keychain" for this test, so
// clean-up of the keychain path is checked too.
func useKeychain(t *testing.T, h *harness) *secret.Memory {
	mem := &secret.Memory{}
	old := secret.Keychain
	secret.Keychain = mem
	t.Cleanup(func() { secret.Keychain = old })
	delete(h.env, "MINDFRIES_TOKEN_STORE")
	return mem
}

func TestStopCleansUp(t *testing.T) {
	for _, store := range []string{"keychain", "file"} {
		t.Run(store, func(t *testing.T) {
			h := newHarness(t)
			if store == "keychain" {
				useKeychain(t, h)
			}
			h.started(fakeapi.SessionOptions{Task: quickTask()})
			l, _ := state.Load(h.home)
			if l.TokenStore != store {
				t.Fatalf("token went to %s", l.TokenStore)
			}
			// Without --yes it asks; "n" changes nothing.
			if _, out := h.run("n\n", "stop"); !strings.Contains(out, "Nothing changed") {
				t.Fatalf("declined stop:\n%s", out)
			}
			exit, out := h.run("", "stop", "--yes")
			if exit != 0 || !strings.Contains(out, "your code stays") {
				t.Fatalf("stop: %d\n%s", exit, out)
			}
			if left := leftovers(h.home, l.SessionID); len(left) > 0 {
				t.Fatalf("left on disk: %v", left)
			}
			if !h.srv.Revoked(l.SessionID) {
				t.Fatal("token not revoked on the server")
			}
			if info, _ := h.srv.SessionInfo(l.SessionID); info.Status != "revoked" {
				t.Fatalf("server status %s", info.Status)
			}
		})
	}
}

func TestSubmitCleansUp(t *testing.T) {
	h := newHarness(t)
	useKeychain(t, h)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask()})
	l, _ := state.Load(h.home)
	writeFile(t, dir, "limiter.py", "def allow(k):\n    return True\n")
	if exit, out := h.run("", "submit", "-m", "x", "--yes"); exit != 0 {
		t.Fatal(out)
	}
	if left := leftovers(h.home, l.SessionID); len(left) > 0 {
		t.Fatalf("left on disk after submit: %v", left)
	}
	if !h.srv.Revoked(l.SessionID) {
		t.Fatal("token not revoked after submit")
	}
}

func TestExpiryCleansUp(t *testing.T) {
	h := newHarness(t)
	useKeychain(t, h)
	h.keepWatching = true
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask(), Duration: 2 * time.Second})
	exit, out := h.run("", "start", code, "--dir", wsDir(t), "--yes")
	if exit != 0 || !strings.Contains(out, "Time's up") {
		t.Fatalf("expiry: %d\n%s", exit, out)
	}
	id := h.srv.OnlySessionID()
	if left := leftovers(h.home, id); len(left) > 0 {
		t.Fatalf("left on disk after expiry: %v", left)
	}
}

func TestStopWithNothingActive(t *testing.T) {
	h := newHarness(t)
	if exit, out := h.run("", "stop"); exit != 0 || !strings.Contains(out, "nothing to stop") {
		t.Fatalf("%d %s", exit, out)
	}
}
