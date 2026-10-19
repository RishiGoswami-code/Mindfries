package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/secret"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
	"github.com/mindfries/v2/cli/internal/workspace"
)

func (a *App) stop(args []string) error {
	fs := a.newFlags("stop [flags]",
		"Ends your assessment without submitting, stops the session watcher and removes this\n"+
			"computer's session credentials. Your code stays where it is. You can't restart it.")
	yes := fs.Bool("yes", false, "don't ask for confirmation")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	l, c, err := a.active()
	if errors.Is(err, errNoSession) {
		fmt.Fprintln(a.Stdout, "There's no active assessment on this computer; nothing to stop.")
		return nil
	}
	if l == nil {
		return err
	}
	if c == nil {
		// The token is already gone; tidy up what's left.
		a.finish(l, nil, watcher.ReasonRevoked)
		fmt.Fprintln(a.Stdout, "Cleaned up this computer's session.")
		return nil
	}
	if !*yes {
		fmt.Fprintln(a.Stdout, "This ends your assessment without submitting it, and it can't be restarted.")
		fmt.Fprintln(a.Stdout, "To hand in your work instead, run 'mindfries submit'.")
		if !a.ui(false).Explicit("End the assessment now?") {
			fmt.Fprintln(a.Stdout, "Nothing changed.")
			return nil
		}
	}
	home, err := a.home()
	if err != nil {
		return err
	}
	dir := state.Dir(home)
	if watcher.Running(dir) {
		_ = watcher.RequestStop(dir, watcher.ReasonStop)
		watcher.WaitStopped(dir, 30*time.Second)
	}
	if q, err := watcher.OpenQueue(dir); err == nil && q.Depth() > 0 {
		watcher.Flush(c, q, l.Limits, 15*time.Second)
	}
	a.finish(l, c, watcher.ReasonStop)
	fmt.Fprintf(a.Stdout, "The assessment has ended. This computer's session credentials are removed; your code stays in %s.\n", l.Workspace)
	return nil
}

// finish ends the session on this computer: it revokes the token on the
// server (unless the server already ended it), makes the locked files
// writable again, and deletes the token and every piece of local session
// state. The candidate's code is left where it is.
func (a *App) finish(l *state.Local, c *api.Client, reason string) {
	if c != nil && reason != watcher.ReasonRevoked {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := c.Revoke(ctx, reason)
		cancel()
		if err != nil && !api.IsAuthFailure(err) {
			fmt.Fprintf(a.Stdout, "Couldn't tell Mindfries (%v); the session will close on the server when its time runs out.\n", err)
		}
	}
	a.forget(l)
}

// forget removes everything local about the session: the token from both
// the keychain and the file, and the session folder.
func (a *App) forget(l *state.Local) {
	if l.Workspace != "" {
		workspace.Unlock(l.Workspace, l.Locked)
	}
	home, err := a.home()
	if err != nil {
		return
	}
	if secret.Keychain != nil {
		_ = secret.Keychain.Delete(l.SessionID)
	}
	_ = secret.File{Dir: state.Dir(home)}.Delete(l.SessionID)
	if err := state.Remove(home); err != nil {
		fmt.Fprintf(a.Stdout, "Couldn't remove %s (%v); delete it by hand.\n", state.Dir(home), err)
	}
}

// leftovers lists any session credential or state still on disk, for the
// clean-up tests.
func leftovers(home, sessionID string) []string {
	var out []string
	if _, err := os.Stat(state.Dir(home)); err == nil {
		out = append(out, state.Dir(home))
	}
	if _, err := os.Stat(filepath.Join(state.Dir(home), "token")); err == nil {
		out = append(out, "token file")
	}
	if secret.Keychain != nil {
		if _, err := secret.Keychain.Get(sessionID); err == nil {
			out = append(out, "keychain entry")
		}
	}
	return out
}
