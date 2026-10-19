package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/secret"
	"github.com/mindfries/v2/cli/internal/state"
)

// startOptions are start's flags.
type startOptions struct {
	api  string
	dir  string
	ide  string
	yes  bool
	code string
}

func (a *App) start(args []string) error {
	fs := a.newFlags("start [setup-code] [flags]",
		"Connects this computer to your assessment, sets up the workspace, opens your editor\n"+
			"and runs the session watcher until you submit or stop. Run it again without a code\n"+
			"to carry on after a restart.")
	var o startOptions
	fs.StringVar(&o.api, "api", "", "V2 API base URL (default: $MINDFRIES_API or the built-in one)")
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
		if o.code == "" {
			return errors.New("start needs the setup code from your Mindfries dashboard: mindfries start <setup-code>")
		}
		code, ok := normalizeCode(o.code)
		if !ok {
			return errors.New("that doesn't look like a setup code (it's three groups of four letters and digits).\nCopy the whole setup command from your dashboard again")
		}
		local, client, err = a.connect(ctx, a.apiURL(o.api), code)
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
	used := a.openEditor(l, o.ide, o.yes)
	if used != l.IDE {
		l.IDE = used
		home, err := a.home()
		if err != nil {
			return err
		}
		if err := state.Save(home, l); err != nil {
			return err
		}
	}
	return nil
}

// forget removes the local session after the server says it's over.
func (a *App) forget(l *state.Local) {
	home, err := a.home()
	if err != nil {
		return
	}
	_ = secret.Open(home, l.TokenStore).Delete(l.SessionID)
	_ = state.Remove(home)
}
