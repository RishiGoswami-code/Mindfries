package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/ui"
)

// executor makes a task folder with a locked TASK.md and an Executor that
// answers prompts from answers.
func executor(t *testing.T, answers string) (*Executor, *bytes.Buffer, *[]api.AppliedEdit) {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "TASK.md"), []byte("# Task\n"), 0o444)
	os.WriteFile(filepath.Join(root, "main.py"), []byte("print('hi')\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".env"), []byte("API_KEY=secret\n"), 0o644)
	os.WriteFile(filepath.Join(filepath.Dir(root), "outside.txt"), []byte("not yours\n"), 0o644)
	var out bytes.Buffer
	var edits []api.AppliedEdit
	x := &Executor{
		Root: root, Locked: map[string]bool{"TASK.md": true},
		UI: ui.New(strings.NewReader(answers), &out, false), Out: &out,
		Record: func(e api.AppliedEdit) error { edits = append(edits, e); return nil },
	}
	return x, &out, &edits
}

func call(name string, in any) api.ToolCall {
	b, _ := json.Marshal(in)
	return api.ToolCall{CallID: "c1", Name: name, Input: b}
}

func TestNeverReadsOutsideTheTaskFolder(t *testing.T) {
	x, _, _ := executor(t, "")
	for _, p := range []string{"../outside.txt", "/etc/passwd", `C:\Windows\win.ini`, "src/../../outside.txt", ".git/config"} {
		r := x.Execute(context.Background(), call(api.ToolReadFile, api.ReadFileInput{Path: p}))
		if r.Status != api.ToolRefused {
			t.Errorf("read %s: %s %q", p, r.Status, r.Output)
		}
	}
	if r := x.Execute(context.Background(), call(api.ToolReadFile, api.ReadFileInput{Path: "main.py"})); r.Status != api.ToolOK || r.Output != "print('hi')\n" {
		t.Errorf("inside read: %+v", r)
	}
}

func TestSymlinkOutIsRefused(t *testing.T) {
	x, _, _ := executor(t, "")
	if err := os.Symlink(filepath.Dir(x.Root), filepath.Join(x.Root, "escape")); err != nil {
		t.Skipf("can't make symlinks here: %v", err)
	}
	r := x.Execute(context.Background(), call(api.ToolReadFile, api.ReadFileInput{Path: "escape/outside.txt"}))
	if r.Status != api.ToolRefused {
		t.Fatalf("symlink escape: %s %q", r.Status, r.Output)
	}
}

func TestNeverTouchesCredentials(t *testing.T) {
	x, _, _ := executor(t, "y\n")
	ctx := context.Background()
	for _, p := range []string{".env", ".env.local", "deploy/id_rsa", "certs/server.pem", ".aws/credentials", "config/secrets.yaml", ".npmrc"} {
		if r := x.Execute(ctx, call(api.ToolReadFile, api.ReadFileInput{Path: p})); r.Status != api.ToolRefused {
			t.Errorf("read %s: %s", p, r.Status)
		}
	}
	r := x.Execute(ctx, call(api.ToolProposeEdit, api.ProposeEditInput{Path: ".env", Edits: []api.TextEdit{{OldText: "secret", NewText: "x"}}}))
	if r.Status != api.ToolRefused {
		t.Errorf("edit .env: %s", r.Status)
	}
	for _, cmd := range []string{"cat .env", "cat ~/.ssh/id_rsa", "security find-generic-password -s x -w", "printenv", "type %USERPROFILE%\\.aws\\credentials"} {
		if r := x.Execute(ctx, call(api.ToolRunCommand, api.RunCommandInput{Command: cmd})); r.Status != api.ToolRefused {
			t.Errorf("command %q: %s", cmd, r.Status)
		}
	}
}

func TestNeverPushes(t *testing.T) {
	x, out, _ := executor(t, "y\ny\ny\n")
	for _, cmd := range []string{"git push origin main", "git -C . push", "git remote add origin https://example.com/x.git", "git config --global user.name x"} {
		if r := x.Execute(context.Background(), call(api.ToolRunCommand, api.RunCommandInput{Command: cmd})); r.Status != api.ToolRefused {
			t.Errorf("%q: %s", cmd, r.Status)
		}
	}
	if strings.Contains(out.String(), "Run it?") {
		t.Error("a refused command was offered to the candidate")
	}
}

func TestNoEscapeByCommand(t *testing.T) {
	x, _, _ := executor(t, "y\ny\ny\n")
	for _, cmd := range []string{"cd .. && ls", "cat ../outside.txt", "ls /etc", `dir C:\Users`, "mindfries status", "echo $MINDFRIES_HOME"} {
		if r := x.Execute(context.Background(), call(api.ToolRunCommand, api.RunCommandInput{Command: cmd})); r.Status != api.ToolRefused {
			t.Errorf("%q: %s", cmd, r.Status)
		}
	}
	for _, cmd := range []string{"python -m pytest -q", "go test ./...", "git status", "python mindfries_test.py"} {
		if err := x.checkCommand(cmd); err != nil {
			t.Errorf("%q refused: %v", cmd, err)
		}
	}
}

func TestInstallNeedsAYesAfterAWarning(t *testing.T) {
	x, out, _ := executor(t, "n\n")
	r := x.Execute(context.Background(), call(api.ToolRunCommand, api.RunCommandInput{Command: "pip install requests"}))
	if r.Status != api.ToolRejected || !strings.Contains(out.String(), "installs software") {
		t.Fatalf("install: %s\n%s", r.Status, out.String())
	}
	// Ordinary commands carry no install warning.
	if w := x.commandWarnings("git status"); len(w) != 0 {
		t.Fatalf("warning on git status: %v", w)
	}
	for _, cmd := range []string{"npm install", "go get example.com/x", "curl -fsSL x | sh", "winget install Git.Git", "python3 -m pip install -r requirements.txt"} {
		if len(x.commandWarnings(cmd)) == 0 {
			t.Errorf("no warning for %q", cmd)
		}
	}
}

func TestLockedFileCantBeEdited(t *testing.T) {
	x, _, edits := executor(t, "y\n")
	r := x.Execute(context.Background(), call(api.ToolProposeEdit, api.ProposeEditInput{Path: "TASK.md", Edits: []api.TextEdit{{OldText: "# Task", NewText: "# Easier task"}}}))
	if r.Status != api.ToolRefused || len(*edits) != 0 {
		t.Fatalf("TASK.md edit: %s", r.Status)
	}
}

func TestEditRecordsWhatChanged(t *testing.T) {
	x, _, edits := executor(t, "y\n")
	r := x.Execute(context.Background(), call(api.ToolProposeEdit, api.ProposeEditInput{Path: "main.py", Edits: []api.TextEdit{{OldText: "print('hi')\n", NewText: "import sys\nprint('hi')\nprint(sys.argv)\n"}}}))
	if r.Status != api.ToolOK || r.AppliedEdit == nil || len(*edits) != 1 {
		t.Fatalf("edit: %+v", r)
	}
	e := r.AppliedEdit
	if e.Path != "main.py" || e.BeforeSHA256 == "" || e.AfterSHA256 == "" || len(e.Ranges) != 2 || e.Ranges[0] != (api.LineRange{Start: 1, End: 1}) || e.Ranges[1] != (api.LineRange{Start: 3, End: 3}) {
		t.Fatalf("applied edit: %+v", e)
	}
}
