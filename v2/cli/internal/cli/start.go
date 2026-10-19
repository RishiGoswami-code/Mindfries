package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
)

// startOptions are start's flags.
type startOptions struct {
	api       string
	dir       string
	ide       string
	yes       bool
	code      string
	noBrowser bool
}

func (a *App) start(args []string) error {
	fs := a.newFlags("start [flags]",
		"Signs you in through your browser, connects this computer to the assessment you\n"+
			"were invited to, sets up the workspace, opens your editor and runs the session\n"+
			"watcher until you submit or stop. Run it again to carry on after a restart.\n"+
			"(A setup code from the dashboard, 'mindfries start <code>', also still works.)")
	var o startOptions
	fs.StringVar(&o.api, "api", "", "V2 API base URL, for development (default: the built-in one)")
	fs.BoolVar(&o.noBrowser, "no-browser", false, "print the sign-in address instead of opening the browser")
	fs.StringVar(&o.dir, "dir", "", "folder for the workspace (default: asks)")
	fs.StringVar(&o.ide, "ide", "", "editor to open: an id such as vscode, pycharm or zed, or 'manual'")
	fs.BoolVar(&o.yes, "yes", false, "accept the defaults instead of asking")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return errors.New("start takes one setup code")
	}
	if len(pos) == 1 {
		o.code = pos[0]
	}
	ctx := context.Background()

	home, err := a.home()
	if err != nil {
		return err
	}
	local, err := state.Load(home)
	var client *api.Client
	switch {
	case err == nil:
		// A session already exists here: carry on with it.
		if o.code != "" {
			fmt.Fprintln(a.Stdout, "An assessment is already set up on this computer; carrying on with it (the new code isn't needed).")
		}
		local, client, err = a.active()
		if err != nil {
			return err
		}
		sess, err := client.Session(ctx)
		if err != nil {
			if api.IsAuthFailure(err) {
				a.forget(local)
				return sessionEnded(err)
			}
			return fmt.Errorf("couldn't reach Mindfries: %w", err)
		}
		if sess.Status != api.StatusActive {
			a.forget(local)
			return fmt.Errorf("this assessment is %s. Your code is still in the workspace folder", sess.Status)
		}
		fmt.Fprintf(a.Stdout, "Carrying on with %s.\n", titleOf(local))
	case errors.Is(err, state.ErrNone):
		code := ""
		if o.code != "" {
			var ok bool
			if code, ok = normalizeCode(o.code); !ok {
				return errors.New("that doesn't look like a setup code (it's three groups of four letters and digits).\nRun 'mindfries start' without it to sign in instead")
			}
		}
		fmt.Fprintf(a.Stdout, "%s\n", policyNotice)
		a.setup.agents = a.checkAgentsBeforeStart(ctx, o.yes)
		if code == "" {
			local, client, err = a.signIn(ctx, a.apiURL(o.api), o.noBrowser)
		} else {
			local, client, err = a.connect(ctx, a.apiURL(o.api), code)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(a.Stdout, "Connected to %s. Your time ends at %s.\n", titleOf(local), local.EndsAt.Local().Format("15:04 on Mon 2 Jan"))
	default:
		return err
	}
	return a.afterConnect(ctx, local, client, o)
}

func titleOf(l *state.Local) string {
	if l.Title != "" {
		return l.Title
	}
	return "your assessment"
}

// afterConnect is the rest of start: the workspace, the editor and the
// watcher.
func (a *App) afterConnect(ctx context.Context, l *state.Local, c *api.Client, o startOptions) error {
	if err := a.setUpWorkspace(ctx, l, c, o); err != nil {
		return err
	}
	home, err := a.home()
	if err != nil {
		return err
	}
	stateDir := state.Dir(home)
	if err := a.connectGitHub(ctx, l, c); err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			return err
		}
		fmt.Fprintf(a.Stdout, "GitHub: %v\n", err)
	}
	alreadyWatching := watcher.Running(stateDir)
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
