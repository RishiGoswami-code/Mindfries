package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/sys"
)

func TestCodingAgentSignals(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	machine := &sys.Fake{
		Env:     map[string]string{"HOME": "/home/c"},
		Procs:   []string{"explorer.exe", "code.exe"},
		Outputs: map[string]string{"/usr/bin/code --list-extensions": "ms-python.python\ngithub.copilot\n"},
	}
	cfg := e.config()
	cfg.Sys = machine.Sys("linux", "amd64")
	cfg.VSCode = "/usr/bin/code"
	wait := e.run(cfg)

	// 2. The editor's AI extensions, reported once at the start.
	e.eventually("extension signal", func() bool { return len(e.ofType("agent_signal.extension")) == 1 })

	// 1. An agent starts mid-session.
	machine.SetProcs("explorer.exe", "code.exe", "claude.exe")
	e.eventually("process signal", func() bool { return len(e.ofType("agent_signal.process")) == 1 })

	// 3. A planted CLAUDE.md, and a commit with an agent trailer.
	e.write("CLAUDE.md", "# Instructions for the agent\n")
	e.eventually("leftover signal", func() bool { return len(e.ofType("agent_signal.leftover")) == 1 })
	r := &gitx.Repo{Dir: e.ws}
	ctx := context.Background()
	r.Run(ctx, "add", "main.py")
	e.write("main.py", "print('fixed')\n")
	r.Run(ctx, "add", "main.py")
	if _, err := r.Run(ctx, "commit", "-q", "-m", "Fix\n\nCo-Authored-By: Claude <noreply@anthropic.com>"); err != nil {
		t.Fatal(err)
	}
	e.eventually("commit event", func() bool { return len(e.ofType("git.commit")) == 1 })

	// 4. Three whole files arriving at once.
	for i := 0; i < 3; i++ {
		e.write(fmt.Sprintf("gen_%d.py", i), strings.Repeat("x = 1\n", 50))
	}
	e.eventually("burst signal", func() bool { return len(e.ofType("file.burst")) == 1 })

	RequestStop(e.stateDir, ReasonStop)
	wait()

	var proc struct {
		Matches []struct{ Agent, ProcessName string } `json:"matches"`
		Phase   string                                `json:"phase"`
	}
	json.Unmarshal(e.ofType("agent_signal.process")[0].Data, &proc)
	if len(proc.Matches) != 1 || proc.Matches[0].Agent != "claude-code" || proc.Phase != "during" {
		t.Errorf("process signal: %+v", proc)
	}
	var ext struct {
		IDE        string
		Extensions []string
	}
	json.Unmarshal(e.ofType("agent_signal.extension")[0].Data, &ext)
	if ext.IDE != "vscode" || len(ext.Extensions) != 1 || ext.Extensions[0] != "github.copilot" {
		t.Errorf("extension signal: %+v", ext)
	}
	var left struct{ Paths []string }
	json.Unmarshal(e.ofType("agent_signal.leftover")[0].Data, &left)
	if len(left.Paths) != 1 || left.Paths[0] != "CLAUDE.md" {
		t.Errorf("leftover signal: %+v", left)
	}
	var commit struct {
		AgentTrailer string `json:"agent_trailer"`
	}
	json.Unmarshal(e.ofType("git.commit")[0].Data, &commit)
	if !strings.Contains(commit.AgentTrailer, "Claude") {
		t.Errorf("commit signal: %+v", commit)
	}

	// Nothing outside the listed checks was read: the only command run is
	// the extension list, and no files were read through the machine.
	for _, rd := range machine.Reads {
		if rd != "run /usr/bin/code --list-extensions" && !strings.HasPrefix(rd, "readdir ") && !strings.HasPrefix(rd, "read ") {
			t.Errorf("unexpected read: %s", rd)
		}
		if strings.HasPrefix(rd, "read") && !strings.Contains(rd, "JetBrains") && !strings.Contains(rd, filepath.Join("zed", "settings.json")) {
			t.Errorf("read outside editor settings: %s", rd)
		}
	}
}

func TestTypingIsNotABurst(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	wait := e.run(e.config())
	body := ""
	for i := 0; i < 30; i++ {
		body += fmt.Sprintf("line_%d = %d\n", i, i)
		e.write("typed.py", body)
		time.Sleep(40 * time.Millisecond)
	}
	e.eventually("typing recorded", func() bool { return len(e.ofType("file.changed")) >= 3 && e.q.Depth() == 0 })
	RequestStop(e.stateDir, ReasonStop)
	wait()
	if n := len(e.ofType("file.burst")); n != 0 {
		t.Fatalf("typing flagged as %d bursts", n)
	}
}
