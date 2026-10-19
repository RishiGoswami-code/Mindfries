package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mindfries/v2/cli/internal/ide"
	"github.com/mindfries/v2/cli/internal/signals"
)

// policyNotice is shown before the session starts: the rules, and exactly
// what this CLI looks at (v2/PLAN.md task 1.8).
const policyNotice = `Before you start
  You may write code yourself and with 'mindfries agent', the assistant this
  assessment provides. Your own coding agents (Claude Code, Gemini CLI, Codex,
  Aider, Copilot and the like) aren't allowed. Looking things up in your
  browser is fine.

  While the session runs, mindfries records:
    - Git state, file changes and diff snapshots inside the workspace folder;
    - the names of running programs that match known coding agents (only the
      matches, never the full list);
    - AI extensions installed in your editor;
    - agent instruction files (like CLAUDE.md) in the workspace, and agent
      trailers in your commit messages.
  It doesn't record keystrokes, your screen, or any file outside the workspace.
  What it finds is shown to a person as something to look at, never as a verdict.
`

// startAgents is what the pre-start check found.
type startAgents struct {
	matches []signals.ProcessMatch
	closed  bool
}

// checkAgentsBeforeStart asks the candidate to close any coding agent
// before the session (and its timer) starts.
func (a *App) checkAgentsBeforeStart(ctx context.Context, yes bool) startAgents {
	found, err := signals.Processes(ctx, a.sys())
	if err != nil || len(found) == 0 {
		return startAgents{}
	}
	first := found
	u := a.ui(yes)
	for round := 0; len(found) > 0; round++ {
		var names []string
		for _, m := range found {
			names = append(names, m.ProcessName)
		}
		if round == 0 {
			fmt.Fprintf(a.Stdout, "\nThese coding agents are running: %s\n", strings.Join(names, ", "))
			fmt.Fprintln(a.Stdout, "Please close them before you start. If you go ahead with them open, that is noted for the reviewer.")
		} else {
			// Answering yes again doesn't help while they're still open.
			fmt.Fprintf(a.Stdout, "\nStill running: %s. Quit them completely (check the system tray too), or answer n to start with them open.\n", strings.Join(names, ", "))
		}
		if yes {
			return startAgents{matches: first}
		}
		if !u.Confirm("Closed them? (no = start anyway)", true) {
			return startAgents{matches: first}
		}
		found, err = signals.Processes(ctx, a.sys())
		if err != nil {
			break
		}
	}
	return startAgents{matches: first, closed: true}
}

// vscodePath is the VS Code launcher, if installed.
func (a *App) vscodePath() string {
	if e, ok := ide.ByID(ide.Detect(a.sys()), "vscode"); ok {
		return e.Path
	}
	return ""
}
