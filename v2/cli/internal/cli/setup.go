package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/buildinfo"
	"github.com/mindfries/v2/cli/internal/preflight"
	"github.com/mindfries/v2/cli/internal/shell"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/trust"
	"github.com/mindfries/v2/cli/internal/ui"
	"github.com/mindfries/v2/cli/internal/workspace"
)

func (a *App) ui(yes bool) *ui.UI {
	if a.prompt == nil {
		a.prompt = ui.New(a.Stdin, a.Stdout, yes)
	}
	a.prompt.Yes = yes
	return a.prompt
}

// setupResult is what this run's setup found, for the watcher's first
// events.
type setupResult struct {
	resumed bool
	checks  []preflight.Check
	agents  startAgents
}

// blocking checks stop setup; the rest can be fixed later.
var blocking = map[string]bool{"os": true, "git": true, "disk": true}

// setUpWorkspace fetches and checks the task, then creates the workspace.
// It is skipped when the workspace from an earlier start is still there.
func (a *App) setUpWorkspace(ctx context.Context, l *state.Local, c *api.Client, o startOptions) error {
	home, err := a.home()
	if err != nil {
		return err
	}
	stateDir := state.Dir(home)
	if workspace.Present(ctx, l) {
		fmt.Fprintf(a.Stdout, "Workspace: %s\n", l.Workspace)
		a.setup.resumed = true
		return nil
	}
	if l.Workspace != "" {
		fmt.Fprintf(a.Stdout, "The workspace folder %s is missing or damaged; setting it up again.\n", l.Workspace)
	}
	u := a.ui(o.yes)

	m, raw, err := c.BundleManifest(ctx)
	if err != nil {
		return fmt.Errorf("couldn't fetch the task: %w", err)
	}
	if err := trust.VerifyManifest(raw, l.APIURL); err != nil {
		return fmt.Errorf("the task couldn't be verified, so nothing was set up: %v.\nTell the hiring team; this isn't something you did", err)
	}

	dir := o.dir
	if dir == "" {
		dir = l.Workspace
	}
	if dir == "" {
		dir = u.Ask("Where should the workspace go?", workspace.DefaultFolder(a.sys().Home(), l.Title))
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}

	checks := preflight.Run(ctx, a.sys(), preflight.Options{Dir: nearestExisting(dir), CLIVersion: buildinfo.Version, Runtimes: m.RequiredRuntimes})
	a.setup.checks = checks
	var problems []preflight.Check
	stop := false
	for _, ch := range checks {
		if ch.Status != preflight.Pass {
			problems = append(problems, ch)
			stop = stop || (ch.Status == preflight.Fail && blocking[ch.Name])
		}
	}
	if len(problems) > 0 {
		fmt.Fprintln(a.Stdout, "\nChecks that need attention:")
		printChecks(a.Stdout, problems)
		fmt.Fprintln(a.Stdout)
	}
	if stop {
		return &ExitError{Code: 1, Err: errors.New("fix what's marked FAIL above, then run 'mindfries start' again. Your session is kept, and none of this counts against you")}
	}
	if preflight.Failed(problems) && !u.Confirm("Some of the task's tools are missing. Carry on and install them later?", true) {
		return &ExitError{Code: 1, Err: errors.New("install what's missing, then run 'mindfries start' again. Your session is kept")}
	}

	switch chk, err := workspace.CheckFolder(dir); {
	case err != nil:
		return err
	case chk == workspace.FolderIsFile:
		return fmt.Errorf("%s is a file, not a folder. Run 'mindfries start --dir <folder>' with another folder", dir)
	case chk == workspace.FolderIsRepo:
		return fmt.Errorf("%s is already a Git repository. Choose an empty folder: mindfries start --dir <folder>", dir)
	case chk == workspace.FolderNotEmpty:
		if !u.Explicit(fmt.Sprintf("%s isn't empty. Files with the same names as the task's will be replaced. Use it anyway?", dir)) {
			return errors.New("choose an empty folder: mindfries start --dir <folder>")
		}
	}

	fmt.Fprintf(a.Stdout, "Downloading %s…\n", m.TaskVersion)
	archive, err := workspace.Download(ctx, c, m, stateDir, a.Stdout)
	if err != nil {
		if errors.Is(err, workspace.ErrTampered) {
			return fmt.Errorf("%w. Nothing was written to %s. Tell the hiring team; this isn't something you did", err, dir)
		}
		return &ExitError{Code: 1, Err: fmt.Errorf("%w.\nCheck your connection (or VPN/proxy) and run 'mindfries start' again: it keeps what was downloaded and carries on. Your session is kept, and none of this counts against you", err)}
	}
	staging := filepath.Join(stateDir, "staging")
	if err := workspace.Extract(archive, staging, m); err != nil {
		os.Remove(archive)
		return fmt.Errorf("%w. Nothing was written to %s. Tell the hiring team; this isn't something you did", err, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := workspace.Install(staging, dir, m); err != nil {
		return err
	}
	base, err := workspace.InitGit(ctx, dir, m)
	if err != nil {
		return fmt.Errorf("setting up Git in %s: %w", dir, err)
	}
	locked, err := workspace.Lock(dir, stateDir, m)
	if err != nil {
		return err
	}
	l.Workspace, l.TaskVersion, l.Branch, l.BaseCommit = dir, m.TaskVersion, m.Git.Branch, base
	l.Locked, l.Runtimes, l.CheckCommands = locked, m.RequiredRuntimes, m.CheckCommands
	if err := state.Save(home, l); err != nil {
		return err
	}
	_ = os.Remove(archive)
	fmt.Fprintf(a.Stdout, "Workspace ready: %s (branch %s)\n", dir, m.Git.Branch)
	if len(locked) > 0 {
		var names []string
		for _, lf := range locked {
			names = append(names, lf.Path)
		}
		fmt.Fprintf(a.Stdout, "%s is read-only: it's the task, and it can't be changed.\n", strings.Join(names, ", "))
	}
	return a.runSetupCommands(ctx, u, dir, stateDir, m.SetupCommands)
}

// runSetupCommands installs the task's dependencies, one confirmed command
// at a time. A failure offers retry or skip, with the log kept.
func (a *App) runSetupCommands(ctx context.Context, u *ui.UI, dir, stateDir string, cmds []string) error {
	for i, cmd := range cmds {
		fmt.Fprintf(a.Stdout, "\nThe task installs its dependencies with:\n  %s\n", cmd)
		if !u.Confirm("Run it now?", true) {
			fmt.Fprintln(a.Stdout, "Skipped. You can run it yourself in the workspace later.")
			continue
		}
		logPath := filepath.Join(stateDir, fmt.Sprintf("setup-%d.log", i+1))
		for {
			var buf bytes.Buffer
			res, err := shell.Run(ctx, dir, cmd, io.MultiWriter(a.Stdout, &buf), 0)
			_ = os.WriteFile(logPath, buf.Bytes(), 0o600)
			if err == nil && res.ExitCode == 0 {
				break
			}
			why := fmt.Sprintf("exit code %d", res.ExitCode)
			if err != nil {
				why = err.Error()
			}
			fmt.Fprintf(a.Stdout, "\nThat failed (%s). The full output is in %s\n", why, logPath)
			if u.Confirm("Try again?", false) {
				continue
			}
			fmt.Fprintln(a.Stdout, "Skipped. This doesn't count against you; tell the hiring team if you can't get it working.")
			break
		}
	}
	return nil
}

// nearestExisting walks up from p to a folder that exists, for measuring
// free space before the workspace is created.
func nearestExisting(p string) string {
	for {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return p
		}
		p = parent
	}
}
