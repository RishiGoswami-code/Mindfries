package workspace

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/mindfries/candidate-backend/internal/sandbox"
)

// Whether the command record can still be trusted (issue #118).
//
// Commands are reported by hooks in the candidate's own shell setup
// (seed.go). The candidate owns the machine, so they can remove the hooks —
// and then commands go unrecorded — or print the hooks' marker themselves,
// and then a command appears that never ran. Neither can be prevented from
// here. What can be done is noticing, and saying so in the report, so a
// hiring team never reads an altered record as a complete one.
//
// Three things are checked:
//
//   - The shell setup files, whenever the work is captured: the hooks are
//     still there as written, and nothing added after them, or in another
//     startup file, takes them back out.
//   - Every command the shell reports: one that touches the hooks or prints
//     the marker is itself recorded before it runs, so removing the hooks
//     from inside a running shell shows up in the record it is removing.
//   - The project's files: a script that prints the marker has to be
//     written somewhere, and the project is what gets captured.
//
// None of this catches everything — a determined candidate can work around
// a check they know about — but each check turns a silent change into a
// visible one.

// markerFingerprints are things that only someone working on the hooks or
// the marker would type or write.
var markerFingerprints = regexp.MustCompile(`7770;|mf_preexec|mf_precmd|mf_ready|add-zsh-hook\s+-d|(pre)?exec_functions|precmd_functions|PROMPT_COMMAND|trap\b[^\n]*\bDEBUG\b`)

// SuspiciousCommand reports whether a command the shell reported touches the
// hooks that report commands, or prints their marker.
func SuspiciousCommand(command string) bool {
	return markerFingerprints.MatchString(command)
}

// The startup files a shell reads besides the two the hooks are written to.
var otherStartupFiles = []string{".zshenv", ".zprofile", ".zlogin", ".bash_profile", ".profile", ".bash_login"}

// CheckCommandHooks reads the shell's startup files and returns what, if
// anything, has been changed that affects the command record. Empty means
// the hooks are as they were set up.
func (w Workspace) CheckCommandHooks(ctx context.Context) ([]string, error) {
	var problems []string
	for name, block := range map[string]string{".zshrc": zshrc, ".bashrc": bashrc} {
		raw, err := w.Client.ReadFile(ctx, w.SandboxID, name)
		if errors.Is(err, sandbox.ErrNotFound) {
			problems = append(problems, "~/"+name+" was deleted, and with it the hooks that report commands")
			continue
		}
		if err != nil {
			return nil, err
		}
		content := string(raw)
		at := strings.Index(content, block)
		if at < 0 {
			problems = append(problems, "the hooks that report commands in ~/"+name+" were edited or removed")
			continue
		}
		if after := content[at+len(block):]; markerFingerprints.MatchString(after) {
			problems = append(problems, "lines added to ~/"+name+" after the hooks change how commands are reported")
		}
	}
	for _, name := range otherStartupFiles {
		raw, err := w.Client.ReadFile(ctx, w.SandboxID, name)
		if errors.Is(err, sandbox.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if markerFingerprints.MatchString(string(raw)) {
			problems = append(problems, "~/"+name+" changes how commands are reported")
		}
	}
	return problems, nil
}

// FilesMentionTheMarker lists project files that contain the hooks' marker
// or name them — something no task needs.
func FilesMentionTheMarker(files map[string]string) []string {
	var out []string
	for path, content := range files {
		if strings.Contains(content, "7770;") || strings.Contains(content, "mf_preexec") || strings.Contains(content, "mf_precmd") {
			out = append(out, path)
		}
	}
	return out
}
