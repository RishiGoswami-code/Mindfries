package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

func (a *App) status(args []string) error {
	fs := a.newFlags("status", "Shows the session, sync and Git state.")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	local, client, err := a.active()
	if errors.Is(err, errNoSession) {
		fmt.Fprintln(a.Stdout, "No active assessment on this computer.")
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
	return nil
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
