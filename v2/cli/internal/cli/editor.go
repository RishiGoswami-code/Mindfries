package cli

import (
	"fmt"
	"strings"

	"github.com/mindfries/v2/cli/internal/ide"
	"github.com/mindfries/v2/cli/internal/state"
)

// openEditor opens the workspace in the candidate's editor. Not finding or
// not launching one is never fatal: the folder can be opened by hand.
// It returns the editor id used, or "manual".
func (a *App) openEditor(l *state.Local, flagIDE string, yes bool) string {
	s := a.sys()
	all := ide.Detect(s)
	supported := ide.Supported(all)
	for _, e := range all {
		if e.AgentFirst {
			fmt.Fprintf(a.Stdout, "%s is installed. It isn't offered for this assessment; please use one of the editors below.\n", e.Name)
		}
	}

	want := flagIDE
	if want == "" {
		want = l.IDE
	}
	var chosen *ide.Editor
	switch {
	case want == string(ide.Manual):
	case want != "":
		if e, ok := ide.ByID(supported, want); ok {
			chosen = &e
		} else if e, ok := ide.ByID(all, want); ok && e.AgentFirst {
			fmt.Fprintf(a.Stdout, "%s isn't offered for this assessment.\n", e.Name)
		} else {
			fmt.Fprintf(a.Stdout, "Couldn't find the editor %q on this computer.\n", want)
		}
		if chosen == nil && len(supported) > 0 {
			chosen = a.pickEditor(supported, yes)
		}
	case len(supported) > 0:
		chosen = a.pickEditor(supported, yes)
	}

	if chosen == nil {
		manualOpen(a, l.Workspace, len(supported) == 0)
		return string(ide.Manual)
	}
	if err := ide.Launch(s, *chosen, l.Workspace); err != nil {
		fmt.Fprintf(a.Stdout, "Couldn't start %s (%v).\n", chosen.Name, err)
		manualOpen(a, l.Workspace, false)
		return string(ide.Manual)
	}
	fmt.Fprintf(a.Stdout, "Opened the workspace in %s.\n", chosen.Name)
	if chosen.Family == ide.VSCode {
		fmt.Fprintln(a.Stdout, "It's a new VS Code window with TASK.md open; if you don't see it, it may be behind this terminal.")
	}
	return chosen.ID
}

func (a *App) pickEditor(eds []ide.Editor, yes bool) *ide.Editor {
	u := a.ui(yes)
	if len(eds) == 1 {
		if u.Confirm(fmt.Sprintf("Open the workspace in %s?", eds[0].Name), true) {
			return &eds[0]
		}
		return nil
	}
	names := make([]string, 0, len(eds)+1)
	for _, e := range eds {
		names = append(names, e.Name)
	}
	names = append(names, "I'll open the folder myself")
	i := u.Choose("Which editor should the workspace open in?", names, 0)
	if i == len(eds) {
		return nil
	}
	return &eds[i]
}

func manualOpen(a *App, dir string, noneFound bool) {
	if noneFound {
		fmt.Fprintln(a.Stdout, "No supported editor (VS Code, a JetBrains IDE or Zed) was found.")
	}
	fmt.Fprintf(a.Stdout, "Open this folder in your editor:\n  %s\n", dir)
	if strings.ContainsAny(dir, " ") {
		fmt.Fprintln(a.Stdout, "(The path has spaces; quote it if you open it from a terminal.)")
	}
}
