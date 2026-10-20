package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/buildinfo"
	"github.com/mindfries/v2/cli/internal/preflight"
	"github.com/mindfries/v2/cli/internal/state"
)

// notStarted ends start before the clock runs: nothing counts against the
// candidate, and running start again picks up from here.
func notStarted(format string, args ...any) error {
	return &ExitError{Code: 1, Err: fmt.Errorf(format+".\nThe assessment hasn't started and your time isn't running; nothing counts against you. Run 'mindfries start' again once it's fixed", args...)}
}

// monitoringWait is how long start waits for the panel.
var monitoringWait = 15 * time.Minute

// begin is start for a session that's set up and waiting: the editor, the
// checks, monitoring, and only then the clock.
func (a *App) begin(ctx context.Context, l *state.Local, c *api.Client, sess *api.Session, o startOptions) error {
	home, err := a.home()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "\n%s\n", policyNotice)
	a.setup.agents = a.checkAgentsBeforeStart(ctx, o.yes)

	// 1. The editor, from the ones installed.
	editor, err := a.chooseEditor(l, o.ide, o.yes)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	// 2. This computer: anything that fails stops the start.
	fmt.Fprintln(a.Stdout, "\nChecking this computer…")
	checks := preflight.Run(ctx, a.sys(), preflight.Options{Dir: l.Workspace, API: c.Version, CLIVersion: buildinfo.Version, Runtimes: l.Runtimes})
	a.setup.checks = checks
	printChecks(a.Stdout, checks)
	if preflight.Failed(checks) {
		return notStarted("\nThis computer isn't ready: fix what's marked FAIL above")
	}

	// 3. Monitoring, in the browser panel.
	if sess.Monitoring.Any() {
		if err := a.waitMonitoring(ctx, c, sess.Monitoring, o); err != nil {
			return err
		}
	}

	a.monitoring = sess.Monitoring

	// 4. The clock.
	req := api.BeginRequest{IDE: "manual"}
	if editor != nil {
		req.IDE = editor.ID
	}
	for _, ch := range checks {
		req.Checks = append(req.Checks, api.BeginCheck{Name: ch.Name, Status: strings.ToLower(strings.TrimSpace(ch.Status.String())), Detail: ch.Detail})
	}
	started, err := c.Begin(ctx, req)
	if api.HasCode(err, api.CodeMonitoringNotReady) {
		// It went off between the check and now: once more.
		if err := a.waitMonitoring(ctx, c, sess.Monitoring, o); err != nil {
			return err
		}
		started, err = c.Begin(ctx, req)
	}
	if err != nil {
		if p, ok := api.AsProblem(err); ok && p.Code == api.CodeMonitoringNotReady {
			return notStarted("monitoring isn't on for: %s", p.Detail)
		}
		return notStarted("couldn't start the assessment with Mindfries (%v)", err)
	}
	l.EndsAt = started.EndsAt
	fmt.Fprintf(a.Stdout, "\nStarted. Your time ends at %s.\n", l.EndsAt.Local().Format("15:04 on Mon 2 Jan"))

	// 5. The editor, in the workspace, and the watcher.
	l.IDE = a.launchEditor(l, editor)
	if err := state.Save(home, l); err != nil {
		return err
	}
	return a.watch(ctx, l, c, l.IDE)
}

// waitMonitoring opens the monitoring panel and waits until everything the
// assessment needs is on. Refusing means the assessment can't start.
func (a *App) waitMonitoring(ctx context.Context, c *api.Client, m *api.Monitoring, o startOptions) error {
	if st, err := c.MonitoringStatus(ctx); err == nil && len(st.Missing(m)) == 0 {
		fmt.Fprintln(a.Stdout, "\nMonitoring is on.")
		return nil
	}
	var needs []string
	if m.Camera {
		needs = append(needs, "camera")
	}
	if m.Screen {
		needs = append(needs, "screen")
	}
	if m.Audio {
		needs = append(needs, "microphone")
	}
	fmt.Fprintf(a.Stdout, "\nThis assessment records your %s while it runs, in a Mindfries tab in your browser:\n", joinAnd(needs))
	var kept []string
	if m.Camera {
		kept = append(kept, "a camera still")
	}
	if m.Screen {
		kept = append(kept, "a screenshot")
	}
	if len(kept) > 0 {
		fmt.Fprintf(a.Stdout, "  %s every 30 seconds", strings.Join(kept, " and "))
		if m.Audio {
			fmt.Fprint(a.Stdout, ", and your microphone in 30-second clips")
		}
		fmt.Fprintln(a.Stdout, ".")
	} else if m.Audio {
		fmt.Fprintln(a.Stdout, "  your microphone in 30-second clips.")
	}
	fmt.Fprintln(a.Stdout, "  It's evidence for the person reviewing your work, never an automatic verdict.")
	if !a.ui(o.yes).Confirm("Turn it on now? (no = don't start)", true) {
		return notStarted("monitoring wasn't turned on, and this assessment needs it")
	}
	a.openURL(m.PanelURL, o.noBrowser)
	fmt.Fprintln(a.Stdout, "Allow access in that tab and keep it open; the assessment starts by itself once everything is on (Ctrl+C to stop).")

	interval := 2 * time.Second
	if a.tunePoll != nil {
		interval = a.tunePoll(interval)
	}
	deadline := time.Now().Add(monitoringWait)
	last := ""
	for {
		st, err := c.MonitoringStatus(ctx)
		if err == nil {
			missing := st.Missing(m)
			if len(missing) == 0 {
				fmt.Fprintln(a.Stdout, "Monitoring is on.")
				return nil
			}
			if now := joinAnd(missing); now != last {
				fmt.Fprintf(a.Stdout, "  Waiting for: %s\n", now)
				last = now
			}
		} else if !api.IsRetryable(err) {
			return notStarted("couldn't check the monitoring panel (%v)", err)
		}
		if time.Now().After(deadline) {
			return notStarted("monitoring wasn't turned on in time")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// openURL opens a Mindfries page in the browser, or prints it.
func (a *App) openURL(u string, noBrowser bool) {
	if !webLink(u) {
		return
	}
	if !noBrowser && a.Getenv("MINDFRIES_NO_BROWSER") != "1" && a.openBrowser(u) {
		fmt.Fprintf(a.Stdout, "  Opened %s\n", u)
		return
	}
	fmt.Fprintf(a.Stdout, "  Open this address in your browser:\n    %s\n", u)
}

// joinAnd is "a", "a and b", "a, b and c".
func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
