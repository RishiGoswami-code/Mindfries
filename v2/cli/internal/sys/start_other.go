//go:build !windows

package sys

import (
	"os/exec"
	"syscall"
)

// startCommand builds the command Start runs, in its own process group so
// Ctrl+C in the candidate's terminal doesn't take the editor down with it.
func startCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}
