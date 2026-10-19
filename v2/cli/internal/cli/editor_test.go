package cli

import (
	"errors"
	fp "path/filepath"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/state"
)

func TestOpensTheOnlyEditor(t *testing.T) {
	h := newHarness(t)
	h.machine.Programs["code"] = "/usr/bin/code"
	dir := h.started(fakeapi.SessionOptions{})
	if len(h.machine.Started) != 1 || h.machine.Started[0] != "/usr/bin/code --new-window "+dir {
		t.Fatalf("started: %q", h.machine.Started)
	}
	l, _ := state.Load(h.home)
	if l.IDE != "vscode" {
		t.Fatalf("ide = %q", l.IDE)
	}
	// A re-run (e.g. after the editor crashed) opens the same editor again.
	h.machine.Programs["zed"] = "/usr/bin/zed"
	if exit, out := h.run("", "start"); exit != 0 {
		t.Fatal(out)
	}
	if len(h.machine.Started) != 2 || !strings.HasPrefix(h.machine.Started[1], "/usr/bin/code ") {
		t.Fatalf("re-run started: %q", h.machine.Started)
	}
}

func TestChoosesAmongEditors(t *testing.T) {
	h := newHarness(t)
	h.machine.Programs["code"] = "/usr/bin/code"
	h.machine.Programs["pycharm"] = "/opt/pycharm"
	dir := wsDir(t)
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{})
	// Folder question, then choice 2 (PyCharm).
	exit, out := h.run(dir+"\n2\n", "start", code)
	if exit != 0 {
		t.Fatal(out)
	}
	if len(h.machine.Started) != 1 || h.machine.Started[0] != "/opt/pycharm "+dir {
		t.Fatalf("started: %q\n%s", h.machine.Started, out)
	}
}

func TestIDEFlagAndManual(t *testing.T) {
	h := newHarness(t)
	h.machine.Programs["code"] = "/usr/bin/code"
	h.machine.Programs["zed"] = "/usr/bin/zed"
	dir := h.started(fakeapi.SessionOptions{}, "--ide", "zed")
	if len(h.machine.Started) != 1 || h.machine.Started[0] != "/usr/bin/zed "+dir {
		t.Fatalf("--ide zed: %q", h.machine.Started)
	}

	h2 := newHarness(t)
	h2.machine.Programs["code"] = "/usr/bin/code"
	dir = wsDir(t)
	exit, out := h2.run("", "start", h2.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", dir, "--ide", "manual", "--yes")
	if exit != 0 || len(h2.machine.Started) != 0 || !strings.Contains(out, "Open this folder in your editor") {
		t.Fatalf("manual: %q\n%s", h2.machine.Started, out)
	}
}

func TestNoEditorFound(t *testing.T) {
	h := newHarness(t)
	exit, out := h.run("", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", wsDir(t), "--yes")
	if exit != 0 || !strings.Contains(out, "No supported editor") || !strings.Contains(out, "Open this folder") {
		t.Fatalf("no editor: %d\n%s", exit, out)
	}
}

func TestLaunchFailureIsNotFatal(t *testing.T) {
	h := newHarness(t)
	h.machine.Programs["code"] = "/usr/bin/code"
	h.machine.StartErr = errors.New("exec format error")
	exit, out := h.run("", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", wsDir(t), "--yes")
	if exit != 0 || !strings.Contains(out, "Couldn't start VS Code") || !strings.Contains(out, "Open this folder") {
		t.Fatalf("launch failure: %d\n%s", exit, out)
	}
}

func TestAgentFirstEditorNotOffered(t *testing.T) {
	h := newHarness(t)
	h.machine.Programs["code"] = "/usr/bin/code"
	h.machine.Programs["cursor"] = "/usr/bin/cursor"
	exit, out := h.run("", "start", h.srv.MintSetupCode(fakeapi.SessionOptions{}), "--dir", wsDir(t), "--ide", "cursor", "--yes")
	if exit != 0 || !strings.Contains(out, "Cursor isn't offered") {
		t.Fatalf("cursor: %d\n%s", exit, out)
	}
	for _, s := range h.machine.Started {
		if strings.Contains(s, "cursor") {
			t.Fatal("launched an agent-first IDE")
		}
	}
}

func wsDir(t *testing.T) string { return fp.Join(t.TempDir(), "ws") }
