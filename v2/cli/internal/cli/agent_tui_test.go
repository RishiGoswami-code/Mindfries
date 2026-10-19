package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/assistant"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/tui"
)

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

// typist types into the TUI once the screen shows what it waits for.
type typist struct {
	t   *testing.T
	w   io.Writer
	out *lockedBuf
}

func (ty typist) after(seen, keys string) {
	ty.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(tui.Strip(ty.out.String()), seen) {
		if time.Now().After(deadline) {
			ty.t.Fatalf("never saw %q on screen:\n%s", seen, tui.Strip(ty.out.String()))
		}
		time.Sleep(10 * time.Millisecond)
	}
	io.WriteString(ty.w, keys)
}

func TestAgentTerminalInterface(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Title: "Rate limiter", Task: quickTask()})
	l, c, err := h.app("", &bytes.Buffer{}).active()
	if err != nil {
		t.Fatal(err)
	}
	sess, err := c.Session(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	inR, inW := io.Pipe()
	out := &lockedBuf{}
	view := assistant.NewTUI(inR, out, func() int { return 160 })
	chat := &assistant.Chat{
		Client: c, View: view, Policy: sess.Policy, Title: l.Title, Workspace: l.Workspace,
		ConvPath: filepath.Join(state.Dir(h.home), "agent.json"),
		Exec:     &assistant.Executor{Root: l.Workspace, View: view},
	}
	done := make(chan error, 1)
	go func() { done <- chat.Run(context.Background()) }()

	ty := typist{t, inW, out}
	ty.after("Welcome to mindfries agent", "/he\t\r")
	ty.after("Ctrl+J     new line", "create notes.md with hello\r")
	ty.after("Do you want to make this edit to notes.md?", "y")
	ty.after("notes.md is updated", "run git --version\r")
	ty.after("Do you want to run this command?", "\r")
	ty.after("exit code 0", "exit\r")
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("agent didn't exit:\n%s", tui.Strip(out.String()))
	}

	screen := tui.Strip(out.String())
	for _, want := range []string{
		"Rate limiter", "cwd: " + dir, // the welcome box
		"/new", "/budget", // help
		"> create notes.md with hello",       // the message echoed
		"⏺ Create(notes.md)", "1 +", "hello", // the diff
		"❯ 1. Yes", "Updated notes.md with 1 addition and 0 removals",
		"⏺ Bash(git --version)", "⎿  git version", "✓ exit code 0",
		"tokens left",
	} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q", want)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "notes.md")); string(b) != "hello\n" {
		t.Fatalf("notes.md = %q", b)
	}
	if t.Failed() {
		t.Logf("screen:\n%s", screen)
	}
}

func TestAgentTerminalInterfaceEscInterrupts(t *testing.T) {
	h := newHarness(t)
	h.started(fakeapi.SessionOptions{Task: quickTask()})
	_, c, _ := h.app("", &bytes.Buffer{}).active()
	sess, _ := c.Session(context.Background())
	inR, inW := io.Pipe()
	out := &lockedBuf{}
	view := assistant.NewTUI(inR, out, func() int { return 80 })
	chat := &assistant.Chat{Client: c, View: view, Policy: sess.Policy, Exec: &assistant.Executor{Root: t.TempDir(), View: view}}
	done := make(chan error, 1)
	go func() { done <- chat.Run(context.Background()) }()
	ty := typist{t, inW, out}
	// Refusing in the menu with Esc leaves the file alone.
	ty.after("Welcome", "create notes.md with hello\r")
	ty.after("Do you want to make this edit", "\x1b")
	ty.after("won't make that change", "\x03\x03")
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("Ctrl+C twice didn't exit:\n%s", tui.Strip(out.String()))
	}
}
