package cli

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/buildinfo"
	"github.com/mindfries/v2/cli/internal/sys"
	"github.com/mindfries/v2/cli/internal/trust"
)

// run executes the CLI with args and stdin, returning the exit code and
// the combined output.
func run(t *testing.T, stdin string, env map[string]string, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	a := &App{
		Stdin:  strings.NewReader(stdin),
		Stdout: &out,
		Stderr: &out,
		Getenv: func(k string) string { return env[k] },
	}
	code := a.Run(args)
	return code, out.String()
}

func TestVersion(t *testing.T) {
	code, out := run(t, "", nil, "--version")
	if code != 0 || !strings.HasPrefix(out, "mindfries ") {
		t.Fatalf("--version: %d %q", code, out)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	code, out := run(t, "", nil, "help")
	if code != 0 {
		t.Fatalf("help exit %d", code)
	}
	for _, c := range []string{"start", "agent", "status", "submit", "stop", "doctor"} {
		if !strings.Contains(out, "  "+c+" ") {
			t.Errorf("help is missing %s:\n%s", c, out)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	code, out := run(t, "", nil, "frobnicate")
	if code != 2 || !strings.Contains(out, "unknown command") {
		t.Fatalf("unknown command: %d %q", code, out)
	}
}

func TestParseInterspersed(t *testing.T) {
	a := &App{Stdout: &bytes.Buffer{}}
	fs := a.newFlags("x", "")
	dir := fs.String("dir", "", "")
	yes := fs.Bool("yes", false, "")
	pos, err := parse(fs, []string{"CODE", "--dir", "d", "--yes", "more"})
	if err != nil || *dir != "d" || !*yes || strings.Join(pos, ",") != "CODE,more" {
		t.Fatalf("parse: %v %q %v %v", err, *dir, *yes, pos)
	}
}

func TestDoctor(t *testing.T) {
	srv := httptest.NewServer(fakeapi.New(fakeapi.Config{}).Handler())
	defer srv.Close()
	f := &sys.Fake{
		Programs: map[string]string{"git": "/usr/bin/git", "code": "/usr/bin/code"},
		Outputs:  map[string]string{"git --version": "git version 2.43.0"},
		Env:      map[string]string{"HOME": "/home/c"},
		Free:     10 << 30,
	}
	var out bytes.Buffer
	// Its own empty home, so a real session on this computer (whose task
	// may need other runtimes) doesn't leak in.
	home := t.TempDir()
	getenv := func(k string) string {
		if k == "MINDFRIES_HOME" {
			return home
		}
		return ""
	}
	a := &App{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, Getenv: getenv, Sys: f.Sys("linux", "amd64")}
	if code := a.Run([]string{"doctor", "--api", srv.URL + fakeapi.Prefix}); code != 0 {
		t.Fatalf("healthy doctor exit %d:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "This machine is ready") {
		t.Fatalf("output:\n%s", out.String())
	}

	delete(f.Programs, "git")
	out.Reset()
	if code := a.Run([]string{"doctor", "--api", srv.URL + fakeapi.Prefix}); code != 1 {
		t.Fatalf("no git: exit %d", code)
	}
	for _, want := range []string{"FAIL  Git", "sudo apt install git", "None of this counts against you"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestDevKeyOptInOnlyForDevBuilds(t *testing.T) {
	defer func() { trust.AllowDevKeyAnywhere = false }()
	env := map[string]string{"MINDFRIES_TRUST_DEV_KEY": "1"}
	if _, out := run(t, "", env, "--version"); !trust.AllowDevKeyAnywhere || !strings.Contains(out, "Only for testing") {
		t.Fatalf("dev build didn't opt in: %q", out)
	}
	old := buildinfo.Version
	buildinfo.Version = "1.0.0"
	defer func() { buildinfo.Version = old }()
	if _, out := run(t, "", env, "--version"); trust.AllowDevKeyAnywhere || !strings.Contains(out, "ignored by release builds") {
		t.Fatalf("release build opted in: %q", out)
	}
	if _, _ = run(t, "", nil, "--version"); trust.AllowDevKeyAnywhere {
		t.Fatal("opted in without the variable")
	}
}

func TestDevKeyBakedIntoDevBuilds(t *testing.T) {
	defer func() { trust.AllowDevKeyAnywhere = false }()
	oldKey, oldVer := buildinfo.TrustDevKey, buildinfo.Version
	defer func() { buildinfo.TrustDevKey, buildinfo.Version = oldKey, oldVer }()
	buildinfo.TrustDevKey = "1"
	if _, out := run(t, "", nil, "--version"); !trust.AllowDevKeyAnywhere || !strings.Contains(out, "trusting the fake API's development key") {
		t.Fatalf("baked-in dev build didn't trust the dev key: %q", out)
	}
	buildinfo.Version = "1.0.0"
	if _, _ = run(t, "", nil, "--version"); trust.AllowDevKeyAnywhere {
		t.Fatal("a release build trusted the dev key")
	}
}
