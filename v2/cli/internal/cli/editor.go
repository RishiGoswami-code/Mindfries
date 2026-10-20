package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mindfries/v2/cli/internal/ide"
	"github.com/mindfries/v2/cli/internal/state"
)

// errNoEditor stops start: the assessment is worked in a local editor.
var errNoEditor = errors.New("no supported editor is installed (VS Code, a JetBrains IDE or Zed).\n" +
	"Install one (VS Code: https://code.visualstudio.com), then run 'mindfries start' again. Your time hasn't started, and none of this counts against you")

// chooseEditor asks which installed editor the assessment opens in (or
// takes --ide). nil with no error means the candidate opens the folder
// themselves (--ide manual).
func (a *App) chooseEditor(l *state.Local, flagIDE string, yes bool) (*ide.Editor, error) {
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
	switch {
	case want == string(ide.Manual):
		return nil, nil
	case want != "":
		if e, ok := ide.ByID(supported, want); ok {
			return &e, nil
		}
		if e, ok := ide.ByID(all, want); ok && e.AgentFirst {
			fmt.Fprintf(a.Stdout, "%s isn't offered for this assessment.\n", e.Name)
		} else {
			fmt.Fprintf(a.Stdout, "Couldn't find the editor %q on this computer.\n", want)
		}
	}
	if len(supported) == 0 {
		return nil, errNoEditor
	}
	return a.pickEditor(supported, yes), nil
}

// pickEditor lists the installed editors and asks for one.
func (a *App) pickEditor(eds []ide.Editor, yes bool) *ide.Editor {
	names := make([]string, 0, len(eds))
	for _, e := range eds {
		names = append(names, e.Name)
	}
	i := a.ui(yes).Choose("Which editor do you want to work in? (these are installed)", names, 0)
	return &eds[i]
}

// launchEditor opens the workspace in e and returns the editor id used, or
// "manual". A launch that fails isn't fatal: the folder can be opened by
// hand.
func (a *App) launchEditor(l *state.Local, e *ide.Editor) string {
	if e == nil {
		manualOpen(a, l.Workspace)
		return string(ide.Manual)
	}
	if err := ide.Launch(a.sys(), *e, l.Workspace); err != nil {
		fmt.Fprintf(a.Stdout, "Couldn't start %s (%v).\n", e.Name, err)
		manualOpen(a, l.Workspace)
		return string(ide.Manual)
	}
	fmt.Fprintf(a.Stdout, "Opened the workspace in %s.\n", e.Name)
	if e.Family == ide.VSCode {
		fmt.Fprintln(a.Stdout, "It's a new VS Code window with TASK.md open; if you don't see it, it may be behind this terminal.")
	}
	return e.ID
}

// openEditor reopens the editor when carrying on with a running session:
// the one chosen before, without asking again.
func (a *App) openEditor(l *state.Local, flagIDE string, yes bool) string {
	e, err := a.chooseEditor(l, flagIDE, yes)
	if err != nil {
		fmt.Fprintln(a.Stdout, "No supported editor was found.")
		manualOpen(a, l.Workspace)
		return string(ide.Manual)
	}
	return a.launchEditor(l, e)
}

func manualOpen(a *App, dir string) {
	fmt.Fprintf(a.Stdout, "Open this folder in your editor:\n  %s\n", dir)
	if strings.ContainsAny(dir, " ") {
		fmt.Fprintln(a.Stdout, "(The path has spaces; quote it if you open it from a terminal.)")
	}
}
