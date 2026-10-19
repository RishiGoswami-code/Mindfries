package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/assistant"
	"github.com/mindfries/v2/cli/internal/state"
)

func (a *App) agent(args []string) error {
	fs := a.newFlags("agent [flags]",
		"Opens mindfries agent, the coding and research assistant for this assessment, in this\n"+
			"terminal. It works while a session started with 'mindfries start' is active.")
	fresh := fs.Bool("new", false, "start a new conversation instead of carrying on the last one")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	l, c, err := a.active()
	if err != nil {
		return err
	}
	if l.Workspace == "" {
		return errors.New("the workspace isn't set up yet: run 'mindfries start' first")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	sess, err := c.Session(ctx)
	switch {
	case api.IsAuthFailure(err):
		return sessionEnded(err)
	case err != nil:
		return fmt.Errorf("couldn't reach Mindfries: %w", err)
	case sess.Status != api.StatusActive:
		return fmt.Errorf("this assessment is %s", sess.Status)
	}
	home, err := a.home()
	if err != nil {
		return err
	}
	locked := map[string]bool{}
	for _, lf := range l.Locked {
		locked[lf.Path] = true
	}
	u := a.ui(false)
	chat := &assistant.Chat{
		Client:          c,
		UI:              u,
		Out:             a.Stdout,
		Policy:          sess.Policy,
		ConvPath:        filepath.Join(state.Dir(home), "agent.json"),
		NewConversation: *fresh,
		Exec: &assistant.Executor{
			Root:   l.Workspace,
			Locked: locked,
			UI:     u,
			Out:    a.Stdout,
			Color:  isTerminal(a.Stdout),
		},
	}
	return chat.Run(ctx)
}
