// Package preflight checks the candidate's machine before an assessment:
// OS, Git, disk space, the API, editors and the task's runtimes. Every
// failure says how to fix it, and none counts against the candidate
// (SPEC §9, "Environment failure handling").
package preflight

import (
	"context"
	"fmt"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/ide"
	"github.com/mindfries/v2/cli/internal/semver"
	"github.com/mindfries/v2/cli/internal/sys"
)

// Status of one check.
type Status int

const (
	Pass Status = iota
	Warn
	Fail
)

func (s Status) String() string {
	return [...]string{"PASS", "WARN", "FAIL"}[s]
}

// Check is one result.
type Check struct {
	Name   string // stable id, e.g. "git", "runtime:python"
	Title  string
	Status Status
	Detail string
	Fix    []string
}

// MinGit is the oldest Git the CLI relies on.
var MinGit = semver.MustParse("2.20.0")

// MinFreeBytes is the free space required where the workspace goes.
const MinFreeBytes = 1 << 30

// Options says what to check.
type Options struct {
	// Dir is where the workspace will go; free space is checked there.
	Dir string
	// API checks that the server can be reached and accepts this CLI.
	// Nil skips the check.
	API func(ctx context.Context) (*api.VersionInfo, error)
	// CLIVersion is this binary's version, for the "newer version" note.
	CLIVersion string
	// Runtimes the task needs, from its manifest.
	Runtimes []api.Runtime
}

// Run performs every check.
func Run(ctx context.Context, s *sys.Sys, o Options) []Check {
	checks := []Check{
		checkOS(s),
		checkGit(ctx, s),
		checkDisk(s, o.Dir),
	}
	if o.API != nil {
		checks = append(checks, checkAPI(ctx, o))
	}
	checks = append(checks, checkIDE(s))
	for _, rt := range o.Runtimes {
		checks = append(checks, checkRuntime(ctx, s, rt))
	}
	return checks
}

// Failed reports whether any check failed.
func Failed(cs []Check) bool {
	for _, c := range cs {
		if c.Status == Fail {
			return true
		}
	}
	return false
}

func checkOS(s *sys.Sys) Check {
	c := Check{Name: "os", Title: "Operating system", Detail: s.GOOS + "/" + s.GOARCH}
	okOS := s.GOOS == "windows" || s.GOOS == "darwin" || s.GOOS == "linux"
	okArch := s.GOARCH == "amd64" || s.GOARCH == "arm64"
	if !okOS || !okArch {
		c.Status = Fail
		c.Fix = []string{
			"Mindfries supports Windows, macOS and Linux on 64-bit Intel/AMD or ARM.",
			"Contact the hiring team before your assessment for another way to take it.",
		}
	}
	return c
}

func checkGit(ctx context.Context, s *sys.Sys) Check {
	c := Check{Name: "git", Title: "Git"}
	if _, err := s.LookPath("git"); err != nil {
		c.Status = Fail
		c.Detail = "not installed"
		c.Fix = gitInstall(s.GOOS)
		return c
	}
	out, err := s.Output(ctx, "", "git", "--version")
	v, ok := semver.Find(out)
	if err != nil || !ok {
		c.Status = Fail
		c.Detail = "installed, but 'git --version' failed"
		c.Fix = append([]string{"Reinstall Git:"}, gitInstall(s.GOOS)...)
		return c
	}
	c.Detail = v.String()
	if semver.Less(v, MinGit) {
		c.Status = Fail
		c.Detail += fmt.Sprintf(" (need %s or newer)", MinGit)
		c.Fix = append([]string{"Update Git:"}, gitInstall(s.GOOS)...)
	}
	return c
}

func gitInstall(goos string) []string {
	switch goos {
	case "windows":
		return []string{"Install Git from https://git-scm.com/download/win (or run: winget install Git.Git), then open a new terminal."}
	case "darwin":
		return []string{"Run: xcode-select --install   (or: brew install git)"}
	default:
		return []string{"Install Git with your package manager, e.g.: sudo apt install git   or   sudo dnf install git"}
	}
}

func checkDisk(s *sys.Sys, dir string) Check {
	c := Check{Name: "disk", Title: "Free disk space"}
	if dir == "" {
		dir = s.Home()
	}
	free, err := s.DiskFree(dir)
	if err != nil {
		c.Status = Warn
		c.Detail = "couldn't measure: " + err.Error()
		return c
	}
	c.Detail = fmt.Sprintf("%.1f GB free", float64(free)/(1<<30))
	if free < MinFreeBytes {
		c.Status = Fail
		c.Fix = []string{fmt.Sprintf("Free up at least %.0f GB on the drive holding %s, or choose another folder.", float64(MinFreeBytes)/(1<<30), dir)}
	}
	return c
}

func checkAPI(ctx context.Context, o Options) Check {
	c := Check{Name: "api", Title: "Mindfries API"}
	v, err := o.API(ctx)
	switch {
	case api.HasCode(err, api.CodeCLITooOld):
		p, _ := api.AsProblem(err)
		c.Status = Fail
		c.Detail = "this mindfries is too old (need " + p.MinimumVersion + ")"
		c.Fix = []string{"Run the setup command from your Mindfries dashboard again; it installs the latest version."}
	case err != nil:
		c.Status = Fail
		c.Detail = "can't reach it: " + err.Error()
		c.Fix = []string{
			"Check your internet connection, VPN or proxy, then try again.",
			"If your network blocks it, tell the hiring team before your assessment starts.",
		}
	default:
		c.Detail = "reachable"
		if cur, ok := semver.Find(o.CLIVersion); ok {
			if latest, ok := semver.Find(v.LatestVersion); ok && semver.Less(cur, latest) {
				c.Status = Warn
				c.Detail = "reachable; a newer mindfries (" + v.LatestVersion + ") is available"
				c.Fix = []string{"Run the setup command from your dashboard again to update."}
			}
		}
	}
	return c
}

func checkIDE(s *sys.Sys) Check {
	c := Check{Name: "ide", Title: "Code editor"}
	all := ide.Detect(s)
	sup := ide.Supported(all)
	var names []string
	for _, e := range sup {
		names = append(names, e.Name)
	}
	if len(sup) == 0 {
		c.Status = Warn
		c.Detail = "no supported editor found"
		c.Fix = []string{
			"Install VS Code (https://code.visualstudio.com), a JetBrains IDE or Zed,",
			"or open the workspace folder by hand in the editor you use.",
		}
	} else {
		c.Detail = strings.Join(names, ", ")
	}
	for _, e := range all {
		if e.AgentFirst {
			c.Fix = append(c.Fix, e.Name+" is installed. It isn't offered for this assessment; use one of the editors above.")
		}
	}
	return c
}

// runtimeProbes are the commands tried for well-known runtimes, in order.
var runtimeProbes = map[string][][]string{
	"python": {{"python3", "--version"}, {"python", "--version"}, {"py", "-3", "--version"}},
	"node":   {{"node", "--version"}},
	"go":     {{"go", "version"}},
	"java":   {{"java", "-version"}},
	"rust":   {{"rustc", "--version"}},
	"docker": {{"docker", "--version"}},
	"dotnet": {{"dotnet", "--version"}},
}

func checkRuntime(ctx context.Context, s *sys.Sys, rt api.Runtime) Check {
	c := Check{Name: "runtime:" + rt.Name, Title: rt.Name + " " + rt.Version}
	probes := runtimeProbes[strings.ToLower(rt.Name)]
	if probes == nil {
		probes = [][]string{{rt.Name, "--version"}}
	}
	install := "Install " + rt.Name + " " + rt.Version + "."
	if rt.InstallURL != "" {
		install += " See " + rt.InstallURL
	}
	for _, p := range probes {
		if _, err := s.LookPath(p[0]); err != nil {
			continue
		}
		out, err := s.Output(ctx, "", p[0], p[1:]...)
		v, ok := semver.Find(strings.TrimPrefix(out, "go version go"))
		if err != nil || !ok {
			continue
		}
		c.Detail = v.String()
		good, cerr := semver.Satisfies(v, rt.Version)
		if cerr != nil {
			c.Status = Warn
			c.Detail += " (the task's version rule couldn't be read)"
			return c
		}
		if !good {
			c.Status = Fail
			c.Detail += " installed, need " + rt.Version
			c.Fix = []string{install}
		}
		return c
	}
	c.Status = Fail
	c.Detail = "not found"
	c.Fix = []string{install}
	return c
}
