package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
)

// startOptions are the flags of setup and start.
type startOptions struct {
	api       string
	dir       string
	ide       string
	yes       bool
	code      string
	noBrowser bool
}

// errNotSetUp is what start says on a computer with nothing set up.
var errNotSetUp = errors.New("nothing is set up on this computer yet.\n" +
	"Press Start assessment on your Mindfries dashboard and run the command it shows\n" +
	"(or run 'mindfries setup' to sign in here), then run 'mindfries start'")

// setup is `mindfries setup [code]`: everything before the clock starts.
// It connects this computer to the assessment (with the dashboard's code,
// or a browser sign-in), downloads and checks the task and its codebase,
// creates the workspace, and connects GitHub, which creates the repository
// and pushes the codebase there. It never starts the time.
func (a *App) setupCmd(args []string) error {
	fs := a.newFlags("setup [code] [flags]",
		"Sets the assessment up on this computer: downloads and checks the task and its\n"+
			"codebase, creates the workspace, and connects your GitHub (the codebase is\n"+
			"pushed to a new private repository there). Your time doesn't start: run\n"+
			"'mindfries start' when you're ready. The code comes from Start assessment on\n"+
			"your dashboard; without one, you sign in through the browser instead.")
	var o startOptions
	fs.StringVar(&o.api, "api", "", "V2 API base URL, for development (default: the built-in one)")
	fs.BoolVar(&o.noBrowser, "no-browser", false, "print addresses instead of opening the browser")
	fs.StringVar(&o.dir, "dir", "", "folder for the workspace (default: asks)")
	fs.BoolVar(&o.yes, "yes", false, "accept the defaults instead of asking")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return errors.New("setup takes one code")
	}
	if len(pos) == 1 {
		o.code = pos[0]
	}
	l, c, err := a.setUp(context.Background(), o)
	if err != nil {
		return err
	}
	a.readyMessage(l)
	_ = c
	return nil
}

// setUp connects (unless this computer already has the session) and does
// everything that needs no clock. It's setup's work, and start's when it's
// given a code.
func (a *App) setUp(ctx context.Context, o startOptions) (*state.Local, *api.Client, error) {
	home, err := a.home()
	if err != nil {
		return nil, nil, err
	}
	l, err := state.Load(home)
	var c *api.Client
	switch {
	case err == nil:
		if o.code != "" {
			fmt.Fprintln(a.Stdout, "An assessment is already set up on this computer; carrying on with it (the new code isn't needed).")
		}
		if l, c, err = a.active(); err != nil {
			return nil, nil, err
		}
		sess, err := c.Session(ctx)
		if err != nil {
			if api.IsAuthFailure(err) {
				a.forget(l)
				return nil, nil, sessionEnded(err)
			}
			return nil, nil, fmt.Errorf("couldn't reach Mindfries: %w", err)
		}
		if sess.Status != api.StatusReady && sess.Status != api.StatusActive {
			a.forget(l)
			return nil, nil, fmt.Errorf("this assessment is %s. Your code is still in the workspace folder", sess.Status)
		}
		l.GitHub = sess.GitHub
	case errors.Is(err, state.ErrNone):
		code := ""
		if o.code != "" {
			var ok bool
			if code, ok = normalizeCode(o.code); !ok {
				return nil, nil, errors.New("that doesn't look like a setup code (it's three groups of four letters and digits).\nCopy the command from your dashboard again, or run 'mindfries setup' to sign in instead")
			}
			l, c, err = a.connect(ctx, a.apiURL(o.api), code)
		} else {
			l, c, err = a.signIn(ctx, a.apiURL(o.api), o.noBrowser)
		}
		if err != nil {
			return nil, nil, err
		}
		fmt.Fprintf(a.Stdout, "Connected to %s.\n", titleOf(l))
	default:
		return nil, nil, err
	}
	if err := a.setUpWorkspace(ctx, l, c, o); err != nil {
		return nil, nil, err
	}
	if err := a.connectGitHub(ctx, l, c); err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			return nil, nil, err
		}
		fmt.Fprintf(a.Stdout, "GitHub: %v\n", err)
	}
	return l, c, nil
}

// readyMessage ends setup: what's done, and what starts the clock.
func (a *App) readyMessage(l *state.Local) {
	fmt.Fprintf(a.Stdout, "\nReady: %s is set up in %s.\n", titleOf(l), l.Workspace)
	mins := "Your time"
	if l.DurationMinutes > 0 {
		mins = fmt.Sprintf("Your %d minutes", l.DurationMinutes)
	}
	fmt.Fprintf(a.Stdout, "%s start only when you run:\n\n    mindfries start\n\n", mins)
	fmt.Fprintln(a.Stdout, "It asks which editor to use, checks this computer, turns on monitoring, then begins.")
}

func (a *App) start(args []string) error {
	fs := a.newFlags("start [flags]",
		"Begins the assessment set up by 'mindfries setup': asks which installed editor to\n"+
			"use, checks this computer, turns on the monitoring the assessment needs (camera,\n"+
			"screen, microphone, in your browser), then starts your time, opens the editor in\n"+
			"the workspace and runs the session watcher until you submit or stop. Nothing\n"+
			"starts if a check fails. Run it again to carry on after a restart.")
	var o startOptions
	fs.StringVar(&o.api, "api", "", "V2 API base URL, for development (default: the built-in one)")
	fs.BoolVar(&o.noBrowser, "no-browser", false, "print addresses instead of opening the browser")
	fs.StringVar(&o.dir, "dir", "", "folder for the workspace, if it isn't set up yet (default: asks)")
	fs.StringVar(&o.ide, "ide", "", "editor to open: an id such as vscode, pycharm or zed, or 'manual'")
	fs.BoolVar(&o.yes, "yes", false, "accept the defaults instead of asking")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return errors.New("start takes at most one setup code")
	}
	if len(pos) == 1 {
		o.code = pos[0]
	}
	ctx := context.Background()
	home, err := a.home()
	if err != nil {
		return err
	}
	if _, err := state.Load(home); errors.Is(err, state.ErrNone) && o.code == "" {
		return errNotSetUp
	}
	// With a code (or a setup that didn't finish), set up first.
	l, c, err := a.setUp(ctx, o)
	if err != nil {
		return err
	}
	sess, err := c.Session(ctx)
	if err != nil {
		return fmt.Errorf("couldn't reach Mindfries: %w", err)
	}
	if sess.Status == api.StatusReady {
		return a.begin(ctx, l, c, sess, o)
	}
	fmt.Fprintf(a.Stdout, "Carrying on with %s.\n", titleOf(l))
	return a.carryOn(ctx, l, c, sess, o)
}

func titleOf(l *state.Local) string {
	if l.Title != "" {
		return l.Title
	}
	return "your assessment"
}

// carryOn is start for a session already running: the editor again, a
// reminder about monitoring, and the watcher.
func (a *App) carryOn(ctx context.Context, l *state.Local, c *api.Client, sess *api.Session, o startOptions) error {
	home, err := a.home()
	if err != nil {
		return err
	}
	a.monitoring = sess.Monitoring
	if sess.Monitoring.Any() {
		if st, err := c.MonitoringStatus(ctx); err == nil {
			if missing := st.Missing(sess.Monitoring); len(missing) > 0 {
				fmt.Fprintf(a.Stdout, "Monitoring is off for: %s. Turn it back on in the panel (the gap is recorded):\n  %s\n", joinAnd(missing), sess.Monitoring.PanelURL)
				a.openURL(sess.Monitoring.PanelURL, o.noBrowser)
			}
		}
	}
	alreadyWatching := watcher.Running(state.Dir(home))
	used := a.openEditor(l, o.ide, o.yes)
	if used != l.IDE {
		l.IDE = used
		if err := state.Save(home, l); err != nil {
			return err
		}
	}
	if alreadyWatching {
		fmt.Fprintln(a.Stdout, "The session watcher is already running in another terminal; nothing more to do here.")
		return nil
	}
	return a.watch(ctx, l, c, used)
}
