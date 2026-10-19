package preflight

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/sys"
)

// healthy is a machine where everything passes.
func healthy() *sys.Fake {
	return &sys.Fake{
		Programs: map[string]string{"git": "/usr/bin/git", "code": "/usr/bin/code", "python3": "/usr/bin/python3"},
		Outputs: map[string]string{
			"git --version":     "git version 2.43.0\n",
			"python3 --version": "Python 3.12.1\n",
		},
		Env:  map[string]string{"HOME": "/home/c"},
		Free: 50 << 30,
	}
}

func find(t *testing.T, cs []Check, name string) Check {
	t.Helper()
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s check in %+v", name, cs)
	return Check{}
}

func okAPI(context.Context) (*api.VersionInfo, error) {
	return &api.VersionInfo{MinimumVersion: "0.1.0", LatestVersion: "0.1.0"}, nil
}

func TestAllPass(t *testing.T) {
	cs := Run(context.Background(), healthy().Sys("linux", "amd64"), Options{
		API: okAPI, CLIVersion: "0.1.0",
		Runtimes: []api.Runtime{{Name: "python", Version: ">=3.11"}},
	})
	for _, c := range cs {
		if c.Status != Pass {
			t.Errorf("%s: %s %s %v", c.Name, c.Status, c.Detail, c.Fix)
		}
	}
	if Failed(cs) {
		t.Fatal("Failed() on a healthy machine")
	}
}

func TestUnsupportedOS(t *testing.T) {
	c := find(t, Run(context.Background(), healthy().Sys("freebsd", "amd64"), Options{}), "os")
	if c.Status != Fail || len(c.Fix) == 0 {
		t.Fatalf("freebsd: %+v", c)
	}
	c = find(t, Run(context.Background(), healthy().Sys("linux", "386"), Options{}), "os")
	if c.Status != Fail {
		t.Fatalf("386: %+v", c)
	}
}

func TestGitMissing(t *testing.T) {
	for _, goos := range []string{"windows", "darwin", "linux"} {
		f := healthy()
		delete(f.Programs, "git")
		c := find(t, Run(context.Background(), f.Sys(goos, "amd64"), Options{}), "git")
		if c.Status != Fail || c.Detail != "not installed" || len(c.Fix) == 0 {
			t.Fatalf("%s: %+v", goos, c)
		}
	}
}

func TestGitTooOld(t *testing.T) {
	f := healthy()
	f.Outputs["git --version"] = "git version 2.17.1"
	c := find(t, Run(context.Background(), f.Sys("linux", "amd64"), Options{}), "git")
	if c.Status != Fail || !strings.Contains(c.Detail, "need 2.20.0") {
		t.Fatalf("old git: %+v", c)
	}
}

func TestDiskLow(t *testing.T) {
	f := healthy()
	f.Free = 200 << 20
	c := find(t, Run(context.Background(), f.Sys("linux", "amd64"), Options{Dir: "/w"}), "disk")
	if c.Status != Fail || len(c.Fix) == 0 {
		t.Fatalf("low disk: %+v", c)
	}
}

func TestAPIUnreachable(t *testing.T) {
	c := find(t, Run(context.Background(), healthy().Sys("linux", "amd64"), Options{
		API: func(context.Context) (*api.VersionInfo, error) { return nil, errors.New("dial tcp: no route") },
	}), "api")
	if c.Status != Fail || len(c.Fix) == 0 {
		t.Fatalf("unreachable: %+v", c)
	}
}

func TestAPICLITooOld(t *testing.T) {
	c := find(t, Run(context.Background(), healthy().Sys("linux", "amd64"), Options{
		API: func(context.Context) (*api.VersionInfo, error) {
			return nil, &api.Problem{Code: api.CodeCLITooOld, Status: 426, MinimumVersion: "0.3.0"}
		},
	}), "api")
	if c.Status != Fail || !strings.Contains(c.Detail, "0.3.0") {
		t.Fatalf("too old: %+v", c)
	}
}

func TestAPINewerAvailable(t *testing.T) {
	c := find(t, Run(context.Background(), healthy().Sys("linux", "amd64"), Options{
		CLIVersion: "0.1.0",
		API: func(context.Context) (*api.VersionInfo, error) {
			return &api.VersionInfo{MinimumVersion: "0.1.0", LatestVersion: "0.2.0"}, nil
		},
	}), "api")
	if c.Status != Warn {
		t.Fatalf("newer: %+v", c)
	}
}

func TestNoEditorIsAWarningNotAFailure(t *testing.T) {
	f := healthy()
	delete(f.Programs, "code")
	cs := Run(context.Background(), f.Sys("linux", "amd64"), Options{})
	c := find(t, cs, "ide")
	if c.Status != Warn || len(c.Fix) == 0 {
		t.Fatalf("no editor: %+v", c)
	}
	if Failed(cs) {
		t.Fatal("a missing editor must not block: the folder can be opened by hand")
	}
}

func TestAgentFirstEditorIsNamed(t *testing.T) {
	f := healthy()
	f.Programs["cursor"] = "/usr/bin/cursor"
	c := find(t, Run(context.Background(), f.Sys("linux", "amd64"), Options{}), "ide")
	if c.Detail != "VS Code" || !strings.Contains(strings.Join(c.Fix, " "), "Cursor") {
		t.Fatalf("cursor: %+v", c)
	}
}

func TestRuntimeMissingAndTooOld(t *testing.T) {
	f := healthy()
	cs := Run(context.Background(), f.Sys("linux", "amd64"), Options{Runtimes: []api.Runtime{
		{Name: "python", Version: ">=3.13"},
		{Name: "node", Version: ">=20", InstallURL: "https://nodejs.org"},
	}})
	py := find(t, cs, "runtime:python")
	if py.Status != Fail || !strings.Contains(py.Detail, "need >=3.13") {
		t.Fatalf("old python: %+v", py)
	}
	node := find(t, cs, "runtime:node")
	if node.Status != Fail || node.Detail != "not found" || !strings.Contains(node.Fix[0], "https://nodejs.org") {
		t.Fatalf("missing node: %+v", node)
	}
}

func TestRuntimeFallsBackToNextProbe(t *testing.T) {
	f := healthy()
	delete(f.Programs, "python3")
	f.Programs["py"] = `C:\Windows\py.exe`
	f.Outputs["py -3 --version"] = "Python 3.11.4"
	c := find(t, Run(context.Background(), f.Sys("windows", "amd64"), Options{Runtimes: []api.Runtime{{Name: "python", Version: ">=3.11"}}}), "runtime:python")
	if c.Status != Pass || c.Detail != "3.11.4" {
		t.Fatalf("py launcher: %+v", c)
	}
}
