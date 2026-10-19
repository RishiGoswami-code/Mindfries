package assistant

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/shell"
	"github.com/mindfries/v2/cli/internal/ui"
	"github.com/mindfries/v2/cli/internal/workspace"
)

// maxOutput caps what a tool sends back to the model.
const maxOutput = 64 << 10

// Executor carries out the assistant's tool calls on the candidate's
// machine, inside the workspace only. Edits and commands need the
// candidate's say-so every time.
type Executor struct {
	Root   string
	Locked map[string]bool
	UI     *ui.UI
	Out    io.Writer
	Color  bool
	// Record is called with every edit that was applied (task 1.17).
	Record func(api.AppliedEdit) error
	// View shows the calls and asks for approval; nil means the plain view
	// on UI and Out.
	View View
}

func (x *Executor) view() View {
	if x.View == nil {
		x.View = &Plain{UI: x.UI, Out: x.Out, Color: x.Color}
	}
	return x.View
}

// refusal is a tool call the CLI won't carry out.
type refusal struct{ why string }

func (r refusal) Error() string { return r.why }

func refuse(format string, args ...any) error { return refusal{fmt.Sprintf(format, args...)} }

// Execute carries out one call and returns its result.
func (x *Executor) Execute(ctx context.Context, c api.ToolCall) api.ToolResult {
	res := api.ToolResult{CallID: c.CallID, Status: api.ToolOK}
	var err error
	switch c.Name {
	case api.ToolReadFile:
		var in api.ReadFileInput
		if err = json.Unmarshal(c.Input, &in); err == nil {
			res.Output, err = x.readFile(in)
		}
	case api.ToolListFiles:
		var in api.ListFilesInput
		if err = json.Unmarshal(c.Input, &in); err == nil {
			res.Output, err = x.listFiles(ctx, in)
		}
	case api.ToolProposeEdit:
		var in api.ProposeEditInput
		if err = json.Unmarshal(c.Input, &in); err == nil {
			res, err = x.proposeEdit(c.CallID, in)
		}
	case api.ToolRunCommand:
		var in api.RunCommandInput
		if err = json.Unmarshal(c.Input, &in); err == nil {
			res, err = x.runCommand(ctx, c.CallID, in)
		}
	default:
		err = refuse("unknown tool %q", c.Name)
	}
	var r refusal
	switch {
	case errors.As(err, &r):
		x.view().Refused(r.why)
		return api.ToolResult{CallID: c.CallID, Status: api.ToolRefused, Output: r.why}
	case err != nil:
		return api.ToolResult{CallID: c.CallID, Status: api.ToolError, Output: err.Error()}
	}
	if len(res.Output) > maxOutput {
		res.Output = res.Output[:maxOutput] + "\n[truncated]"
	}
	return res
}

// resolve turns a workspace path from the model into a real path inside
// the workspace.
func (x *Executor) resolve(p string) (string, error) {
	p = strings.TrimPrefix(filepath.ToSlash(p), "./")
	if !workspace.ValidPath(p) {
		return "", refuse("%q isn't a path inside the task folder", p)
	}
	if isCredentialPath(p) {
		return "", refuse("%s looks like a credentials file, which is off limits", p)
	}
	full := filepath.Join(x.Root, filepath.FromSlash(p))
	ok, err := inside(x.Root, full)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", refuse("%s leads outside the task folder", p)
	}
	return full, nil
}

func (x *Executor) readFile(in api.ReadFileInput) (string, error) {
	full, err := x.resolve(in.Path)
	if err != nil {
		return "", err
	}
	x.view().ToolRead(in.Path)
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	x.view().ToolReadDone(in.Path, string(b))
	return string(b), nil
}

func (x *Executor) listFiles(ctx context.Context, in api.ListFilesInput) (string, error) {
	args := []string{"ls-files", "--cached", "--others", "--exclude-standard"}
	if in.Dir != "" {
		if _, err := x.resolve(in.Dir); err != nil {
			return "", err
		}
		args = append(args, "--", in.Dir)
	}
	x.view().ToolList(in.Dir)
	out, err := (&gitx.Repo{Dir: x.Root}).Run(ctx, args...)
	if err == nil {
		x.view().ToolListDone(out)
	}
	return out, err
}

func newID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (x *Executor) proposeEdit(callID string, in api.ProposeEditInput) (api.ToolResult, error) {
	full, err := x.resolve(in.Path)
	if err != nil {
		return api.ToolResult{}, err
	}
	if x.Locked[in.Path] {
		return api.ToolResult{}, refuse("%s is the task and can't be changed", in.Path)
	}
	beforeBytes, readErr := os.ReadFile(full)
	exists := readErr == nil
	before := string(beforeBytes)
	after := before
	switch {
	case in.Create && exists && before != "":
		return api.ToolResult{}, fmt.Errorf("%s already exists; propose an edit to it instead", in.Path)
	case !in.Create && !exists:
		return api.ToolResult{}, fmt.Errorf("%s doesn't exist", in.Path)
	}
	for _, e := range in.Edits {
		switch n := strings.Count(after, e.OldText); {
		case e.OldText == "" && (after == "" || in.Create):
			after = e.NewText
		case n == 0:
			return api.ToolResult{}, fmt.Errorf("the text to replace wasn't found in %s", in.Path)
		case n > 1:
			return api.ToolResult{}, fmt.Errorf("the text to replace appears %d times in %s; include more of it", n, in.Path)
		default:
			after = strings.Replace(after, e.OldText, e.NewText, 1)
		}
	}
	if in.Create && after != "" && !strings.HasSuffix(after, "\n") {
		after += "\n"
	}
	x.view().ShowEdit(in.Path, in.Create, before, after)
	if !x.view().ConfirmEdit(in.Path) {
		return api.ToolResult{CallID: callID, Status: api.ToolRejected}, nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return api.ToolResult{}, err
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(full); err == nil {
		mode = fi.Mode().Perm()
	}
	edit := api.AppliedEdit{EditID: newID("edit_"), Path: in.Path, AfterSHA256: workspace.HashBytes([]byte(after))}
	if exists {
		edit.BeforeSHA256 = workspace.HashBytes(beforeBytes)
	}
	for _, r := range addedRanges(before, after) {
		edit.Ranges = append(edit.Ranges, api.LineRange{Start: r[0], End: r[1]})
	}
	// Recorded before the write, so the watcher never sees the change
	// without its record.
	if x.Record != nil {
		if err := x.Record(edit); err != nil {
			fmt.Fprintf(x.view().CommandOutput(), "  (couldn't record the edit: %v)\n", err)
		}
	}
	if err := os.WriteFile(full, []byte(after), mode); err != nil {
		return api.ToolResult{}, err
	}
	x.view().Applied(in.Path, before, after)
	return api.ToolResult{CallID: callID, Status: api.ToolOK, AppliedEdit: &edit}, nil
}

func (x *Executor) runCommand(ctx context.Context, callID string, in api.RunCommandInput) (api.ToolResult, error) {
	dir := x.Root
	if in.Cwd != "" {
		d, err := x.resolve(in.Cwd)
		if err != nil {
			return api.ToolResult{}, err
		}
		dir = d
	}
	if err := x.checkCommand(in.Command); err != nil {
		return api.ToolResult{}, err
	}
	if !x.view().ConfirmCommand(in.Command, rel(x.Root, dir), x.commandWarnings(in.Command)) {
		return api.ToolResult{CallID: callID, Status: api.ToolRejected}, nil
	}
	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if timeout <= 0 || timeout > 10*time.Minute {
		timeout = 2 * time.Minute
	}
	var buf bytes.Buffer
	res, err := shell.Run(ctx, dir, in.Command, io.MultiWriter(x.view().CommandOutput(), &buf), timeout)
	if err != nil {
		return api.ToolResult{}, err
	}
	x.view().CommandDone(res.ExitCode, res.TimedOut)
	out := buf.String()
	if len(out) > maxOutput {
		out = "[earlier output cut]\n" + out[len(out)-maxOutput:]
	}
	if res.TimedOut {
		out += "\n[timed out]"
	}
	code := res.ExitCode
	return api.ToolResult{CallID: callID, Status: api.ToolOK, Output: out, ExitCode: &code}, nil
}

func rel(root, dir string) string {
	if r, err := filepath.Rel(root, dir); err == nil && r != "." {
		return "the task folder/" + filepath.ToSlash(r)
	}
	return "the task folder"
}

// checkCommand refuses commands the assistant may never run.
func (x *Executor) checkCommand(cmd string) error {
	for _, b := range blockedCommands {
		if b.re.MatchString(cmd) {
			return refuse("%s", b.why)
		}
	}
	return nil
}

// commandWarnings are shown before the candidate decides.
func (x *Executor) commandWarnings(cmd string) []string {
	var w []string
	if installCommands.MatchString(cmd) {
		w = append(w, "Warning: this downloads and installs software on your computer. Say no unless you want that.")
	}
	return w
}
