package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
)

func (a *App) status(args []string) error {
	fs := a.newFlags("status", "Shows the session, sync and Git state.")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	local, client, err := a.active()
	if errors.Is(err, errNoSession) {
		fmt.Fprintln(a.Stdout, "No active assessment on this computer.")
		if home, err := a.home(); err == nil {
			if last, err := state.LoadLast(home); err == nil {
				fmt.Fprintf(a.Stdout, "Last submitted: %s at %s (commit %s, submission %s), verified by Mindfries.\n",
					orDefault(last.Title, "your assessment"), last.SubmittedAt.Local().Format("15:04 on Mon 2 Jan"), short(last.HeadCommit), last.SubmissionID)
			}
		}
		return nil
	}
	if err != nil {
		return err
	}
	ctx := context.Background()
	fmt.Fprintf(a.Stdout, "Assessment   %s\n", titleOf(local))
	sess, err := client.Session(ctx)
	switch {
	case api.IsAuthFailure(err):
		fmt.Fprintf(a.Stdout, "Session      %s\n", sessionEnded(err))
		return nil
	case err != nil:
		fmt.Fprintf(a.Stdout, "Session      can't reach Mindfries right now (%v)\n", err)
	default:
		left := time.Until(sess.EndsAt).Round(time.Minute)
		if left < 0 {
			left = 0
		}
		fmt.Fprintf(a.Stdout, "Session      %s, %s left (ends %s)\n", sess.Status, left, sess.EndsAt.Local().Format("15:04"))
		fmt.Fprintf(a.Stdout, "Assistant    %s\n", describeMode(sess.Policy))
	}
	if local.Workspace == "" {
		fmt.Fprintln(a.Stdout, "Workspace    not set up yet: run 'mindfries start'")
		return nil
	}
	fmt.Fprintf(a.Stdout, "Workspace    %s\n", local.Workspace)
	a.printWatcher(local)
	r := &gitx.Repo{Dir: local.Workspace}
	if b, err := r.Branch(ctx); err == nil {
		n, _ := r.ChangedFiles(ctx)
		commits, _ := r.Line(ctx, "rev-list", "--count", local.BaseCommit+"..HEAD")
		fmt.Fprintf(a.Stdout, "Git          branch %s, %s commit(s) since the start, %d file(s) changed and not committed\n", b, commits, n)
	}
	return nil
}

func (a *App) printWatcher(local *state.Local) {
	home, err := a.home()
	if err != nil {
		return
	}
	dir := state.Dir(home)
	st, err := watcher.ReadStatus(dir)
	switch {
	case err != nil:
		fmt.Fprintln(a.Stdout, "Watcher      hasn't run yet: run 'mindfries start'")
		return
	case watcher.Running(dir):
		fmt.Fprint(a.Stdout, "Watcher      running")
	default:
		fmt.Fprint(a.Stdout, "Watcher      not running: run 'mindfries start' to carry on")
	}
	switch {
	case st.OfflineSince != nil:
		fmt.Fprintf(a.Stdout, "; offline since %s", st.OfflineSince.Local().Format("15:04:05"))
	case st.LastSyncedAt != nil:
		fmt.Fprintf(a.Stdout, "; last synced %s", st.LastSyncedAt.Local().Format("15:04:05"))
	}
	if st.QueueDepth > 0 {
		fmt.Fprintf(a.Stdout, "; %d event(s) waiting to send", st.QueueDepth)
	}
	fmt.Fprintln(a.Stdout)
}

func describeMode(p api.Policy) string {
	s := map[string]string{
		api.ModeOff:         "off for this assessment",
		api.ModeExplainOnly: "explain only (it won't write code)",
		api.ModeCodeAllowed: "can write code with your approval",
	}[p.AssistantMode]
	if s == "" {
		s = p.AssistantMode
	}
	if b := p.AssistantBudget; b != nil && p.AssistantMode != api.ModeOff {
		s += fmt.Sprintf(", %d of %d tokens left", b.TokensRemaining, b.TokensTotal)
	}
	return s
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
