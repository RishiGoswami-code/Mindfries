package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/preflight"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
)

// watch records this run's setup, then runs the session watcher in the
// foreground until the session ends or the candidate presses Ctrl+C.
func (a *App) watch(ctx context.Context, l *state.Local, c *api.Client, editor string) error {
	home, err := a.home()
	if err != nil {
		return err
	}
	dir := state.Dir(home)
	q, err := watcher.OpenQueue(dir)
	if err != nil {
		return err
	}
	if len(a.setup.checks) > 0 {
		var checks []map[string]any
		for _, ch := range a.setup.checks {
			checks = append(checks, map[string]any{"name": ch.Name, "ok": ch.Status != preflight.Fail, "detail": ch.Detail})
		}
		_, _ = q.Append("preflight.result", map[string]any{"checks": checks})
	}
	_, _ = q.Append("workspace.ready", map[string]any{"task_version": l.TaskVersion, "base_commit": l.BaseCommit, "resumed": a.setup.resumed})
	_, _ = q.Append("ide.launched", map[string]any{"ide": editor, "ok": true})
	if ag := a.setup.agents; len(ag.matches) > 0 {
		_, _ = q.Append("agent_signal.process", map[string]any{"matches": ag.matches, "phase": "start", "closed_by_candidate": ag.closed})
	}

	if prev, err := watcher.ReadStatus(dir); err == nil && !prev.Stopped && time.Since(prev.UpdatedAt) > watcher.StaleAfter {
		fmt.Fprintf(a.Stdout, "The session watcher stopped unexpectedly at %s. Your files and Git history are as you left them.\n", prev.UpdatedAt.Local().Format("15:04"))
		fmt.Fprintln(a.Stdout, "The time it wasn't running is recorded as a gap in the record; it isn't treated as anything you did.")
	}

	cfg := watcher.Config{Client: c, StateDir: dir, Local: l, Queue: q, Out: a.Stdout, Live: isTerminal(a.Stdout), Sys: a.sys(), VSCode: a.vscodePath(), Monitoring: a.monitoring}
	if a.tuneWatcher != nil {
		a.tuneWatcher(&cfg)
	}
	fmt.Fprintln(a.Stdout, "\nThe session watcher is running. Keep this terminal open while you work;")
	fmt.Fprintln(a.Stdout, "use another terminal (or your editor's) for everything else, including 'mindfries agent'.")

	sctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	reason, err := watcher.New(cfg).Run(sctx)
	if errors.Is(err, watcher.ErrAlreadyRunning) {
		fmt.Fprintln(a.Stdout, err.Error()+"; nothing more to do here.")
		return nil
	}
	if err != nil {
		return err
	}
	return a.watcherStopped(l, c, reason)
}

// watcherStopped says what happened and tidies up after a session that
// ended while being watched.
func (a *App) watcherStopped(l *state.Local, c *api.Client, reason string) error {
	switch reason {
	case watcher.ReasonInterrupted:
		fmt.Fprintln(a.Stdout, "\nPaused. Your session is still running and your work is saved.")
		fmt.Fprintln(a.Stdout, "Run 'mindfries start' to carry on watching; the time it wasn't running is recorded as a gap.")
		return nil
	case watcher.ReasonStop, watcher.ReasonSubmitted:
		// The command that asked for the stop finishes the clean-up.
		fmt.Fprintln(a.Stdout, "\nThe session watcher has stopped.")
		return nil
	case watcher.ReasonExpired:
		a.finish(l, c, watcher.ReasonExpired)
		fmt.Fprintln(a.Stdout, "\nTime's up: this assessment has ended. Your code is still in", l.Workspace)
		return nil
	default:
		a.forget(l)
		fmt.Fprintln(a.Stdout, "\nThis assessment session was ended. Your code is still in", l.Workspace)
		return nil
	}
}

func isTerminal(w any) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
