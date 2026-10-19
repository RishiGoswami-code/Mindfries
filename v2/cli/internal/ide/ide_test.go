package ide

import (
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/internal/sys"
)

func TestWindowsVSCodeStartsTheEditorNotTheBatchFile(t *testing.T) {
	cmd := `C:\Users\Jo Doe\AppData\Local\Programs\Microsoft VS Code\bin\code.cmd`
	exe := `C:\Users\Jo Doe\AppData\Local\Programs\Microsoft VS Code\Code.exe`
	f := &sys.Fake{Files: map[string]bool{exe: true}}
	s := f.Sys("windows", "amd64")
	ws := `C:\Users\Jo Doe\mindfries\task`
	if err := Launch(s, Editor{ID: "vscode", Family: VSCode, Path: cmd}, ws); err != nil {
		t.Fatal(err)
	}
	if len(f.Started) != 1 || !strings.HasPrefix(f.Started[0], exe+" --new-window "+ws) {
		t.Fatalf("started %q", f.Started)
	}
	// Without Code.exe next to it, the batch file is still used.
	f2 := &sys.Fake{}
	_ = Launch(f2.Sys("windows", "amd64"), Editor{ID: "vscode", Family: VSCode, Path: cmd}, ws)
	if len(f2.Started) != 1 || !strings.HasPrefix(f2.Started[0], cmd) {
		t.Fatalf("fallback started %q", f2.Started)
	}
}
