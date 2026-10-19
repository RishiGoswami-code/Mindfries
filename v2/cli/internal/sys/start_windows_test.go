//go:build windows

package sys

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A batch file in a folder with spaces, given an argument with spaces,
// gets that argument intact: the case that kept VS Code from opening.
func TestStartRunsBatchFilesWithSpacesInPaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Jo Doe", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "launch.cmd")
	out := filepath.Join(filepath.Dir(dir), "got arg.txt")
	if err := os.WriteFile(script, []byte("@echo %~1> \""+out+"\"\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	arg := filepath.Join(filepath.Dir(dir), "my workspace")
	cmd := startCommand(script, arg)
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v (%s)", err, cmd.SysProcAttr.CmdLine)
	}
	b, err := os.ReadFile(out)
	if err != nil || strings.TrimSpace(string(b)) != arg {
		t.Fatalf("the script got %q (%v)", b, err)
	}
	// And a plain program still runs normally.
	if _, err := exec.LookPath("cmd.exe"); err == nil {
		if err := startCommand("cmd.exe", "/c", "exit 0").Run(); err != nil {
			t.Fatal(err)
		}
	}
}
