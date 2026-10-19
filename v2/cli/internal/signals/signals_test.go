package signals

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/internal/sys"
)

func TestMatchProcesses(t *testing.T) {
	got := MatchProcesses([]string{"explorer.exe", "claude.exe", "node", "Cursor Helper (Renderer)", "Cursor", "aider", "python3", "codex", "cursorless"})
	want := []ProcessMatch{
		{Agent: "aider", ProcessName: "aider"},
		{Agent: "claude-code", ProcessName: "claude.exe"},
		{Agent: "codex", ProcessName: "codex"},
		{Agent: "cursor", ProcessName: "Cursor Helper (Renderer)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if len(MatchProcesses([]string{"code", "idea64.exe", "zed", "git", "bash"})) != 0 {
		t.Fatal("ordinary tools matched")
	}
}

func TestExtensions(t *testing.T) {
	f := &sys.Fake{
		Env: map[string]string{"HOME": "/home/c"},
		Outputs: map[string]string{
			"/usr/bin/code --list-extensions": "ms-python.python\nGitHub.copilot\ngithub.copilot-chat\nesbenp.prettier-vscode\n",
		},
		Dirs: map[string][]string{
			filepath.Join("/home/c", ".local", "share", "JetBrains"):                  {"PyCharm2025.2"},
			filepath.Join("/home/c", ".local", "share", "JetBrains", "PyCharm2025.2"): {"ml-llm", "python-ce", "org.jetbrains.junie"},
		},
		Data: map[string]string{filepath.Join("/home/c", ".config", "zed", "settings.json"): `{ "agent": { "default_model": { "provider": "anthropic" } } }`},
	}
	got := Extensions(context.Background(), f.Sys("linux", "amd64"), "/usr/bin/code")
	want := []Extension{
		{IDE: "vscode", Extensions: []string{"github.copilot", "github.copilot-chat"}},
		{IDE: "jetbrains", Extensions: []string{"ml-llm", "org.jetbrains.junie"}},
		{IDE: "zed", Extensions: []string{"agent-configured"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	// Only the listed sources were read.
	for _, r := range f.Reads {
		ok := r == "run /usr/bin/code --list-extensions" ||
			strings.HasPrefix(r, "readdir "+filepath.Join("/home/c", ".local", "share", "JetBrains")) ||
			strings.HasPrefix(r, "readdir "+filepath.Join("/home/c", ".config", "JetBrains")) ||
			r == "read "+filepath.Join("/home/c", ".config", "zed", "settings.json")
		if !ok {
			t.Errorf("read something outside the listed checks: %s", r)
		}
	}
}

func TestLeftovers(t *testing.T) {
	paths := []string{"src/main.py", "CLAUDE.md", ".claude/settings.json", ".claude/commands/x.md", "docs/AGENTS.md", ".github/copilot-instructions.md", ".aider.chat.history.md", "README.md", ".github/workflows/ci.yml"}
	var hits []string
	for _, p := range paths {
		if IsLeftover(p) {
			hits = append(hits, p)
		}
	}
	got := LeftoverRoots(hits)
	want := []string{".aider.chat.history.md", ".claude", ".github/copilot-instructions.md", "CLAUDE.md", "docs/AGENTS.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestAgentTrailer(t *testing.T) {
	for msg, want := range map[string]string{
		"Fix bug\n\nCo-Authored-By: Claude <noreply@anthropic.com>": "Co-Authored-By: Claude <noreply@anthropic.com>",
		"Add tests\n\n🤖 Generated with some tool":                   "🤖 Generated with some tool",
		"Refactor\n\nCo-authored-by: Sam Lee <sam@example.com>":     "",
		"plain message": "",
	} {
		if got := AgentTrailer(msg); got != want {
			t.Errorf("AgentTrailer(%q) = %q, want %q", msg, got, want)
		}
	}
}

func TestBurst(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	d := NewBurstDetector()
	// Typing: small edits spread over a minute is never a burst.
	for i := 0; i < 60; i++ {
		if b := d.Observe(Arrival{Path: "a.py", Lines: 2, At: t0.Add(time.Duration(i) * time.Second)}); b != nil {
			t.Fatalf("typing flagged at %d: %+v", i, b)
		}
	}
	// Three whole files within two seconds.
	t1 := t0.Add(5 * time.Minute)
	var b *Burst
	for i, p := range []string{"x.py", "y.py", "z.py"} {
		b = d.Observe(Arrival{Path: p, Lines: 40, At: t1.Add(time.Duration(i) * time.Second)})
	}
	if b == nil || len(b.Paths) != 3 || b.LinesAdded != 120 {
		t.Fatalf("files burst: %+v", b)
	}
	// One huge paste.
	if b := d.Observe(Arrival{Path: "big.py", Lines: 200, At: t1.Add(time.Minute)}); b == nil || b.Paths[0] != "big.py" {
		t.Fatalf("paste: %+v", b)
	}
}
