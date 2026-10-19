package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/buildinfo"
	"github.com/mindfries/v2/cli/internal/preflight"
)

func (a *App) doctor(args []string) error {
	fs := a.newFlags("doctor [flags]", "Checks this machine is ready for a Mindfries assessment. Changes nothing.")
	apiURL := fs.String("api", "", "V2 API base URL (default: $MINDFRIES_API or the built-in one)")
	dir := fs.String("dir", "", "folder the workspace will go in (default: your home folder)")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	ctx := context.Background()
	opts := preflight.Options{Dir: *dir, CLIVersion: buildinfo.Version}
	client := a.client(*apiURL)
	opts.API = func(ctx context.Context) (*api.VersionInfo, error) { return client.Version(ctx) }
	opts.Runtimes = a.activeRuntimes()

	checks := preflight.Run(ctx, a.sys(), opts)
	printChecks(a.Stdout, checks)
	if preflight.Failed(checks) {
		fmt.Fprintln(a.Stdout, "\nNone of this counts against you. Fix what's marked FAIL, then run 'mindfries doctor' again.")
		return &ExitError{Code: 1}
	}
	fmt.Fprintln(a.Stdout, "\nThis machine is ready.")
	return nil
}

func printChecks(w io.Writer, checks []preflight.Check) {
	width := 0
	for _, c := range checks {
		if len(c.Title) > width {
			width = len(c.Title)
		}
	}
	for _, c := range checks {
		fmt.Fprintf(w, "  %s  %-*s  %s\n", c.Status, width, c.Title, c.Detail)
		for _, f := range c.Fix {
			fmt.Fprintf(w, "        %s%s\n", strings.Repeat(" ", width+2), f)
		}
	}
}

// activeRuntimes is extended once a session's manifest is stored locally.
var activeRuntimesHook func(a *App) []api.Runtime

func (a *App) activeRuntimes() []api.Runtime {
	if activeRuntimesHook == nil {
		return nil
	}
	return activeRuntimesHook(a)
}
