//go:build windows

package sys

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	createNoWindow        = 0x08000000
)

// startCommand builds the command Start runs. A launched program gets its
// own process group, so Ctrl+C in the candidate's terminal (which pauses
// the watcher) doesn't reach it, and nothing it does reaches us.
//
// A .cmd or .bat launcher (VS Code's code.cmd, JetBrains Toolbox scripts)
// runs through cmd.exe, which mangles a quoted script path followed by a
// quoted argument with spaces ("C:\Users\Jo Doe\…") unless the whole line
// is quoted once more and /s is given. Go's own handling leaves that case
// broken, so the line is written out here.
func startCommand(name string, args ...string) *exec.Cmd {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".cmd", ".bat":
		comspec := os.Getenv("ComSpec")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		parts := []string{quoteCmd(name)}
		for _, a := range args {
			parts = append(parts, quoteCmd(a))
		}
		cmd := exec.Command(comspec)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine:       quoteCmd(comspec) + ` /d /s /c "` + strings.Join(parts, " ") + `"`,
			CreationFlags: createNewProcessGroup | createNoWindow,
		}
		return cmd
	}
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
	return cmd
}

// quoteCmd quotes one word for a cmd.exe line. Double quotes can't appear
// in Windows paths, so they're dropped rather than escaped.
func quoteCmd(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, "") + `"`
}
