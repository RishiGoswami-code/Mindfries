package assistant

import (
	"fmt"
	"io"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/ui"
)

// Welcome is what the agent shows when it opens.
type Welcome struct {
	Title     string // the assessment
	Workspace string
	Mode      string // modeText
	Budget    *api.Budget
}

// View is how `mindfries agent` talks to the candidate: the plain one for
// pipes and scripts (and the tests), or the terminal interface (TUI).
type View interface {
	Welcome(w Welcome)
	// Read returns the next message; ok is false when the candidate leaves.
	Read() (msg string, ok bool)
	Notice(msg string)
	Help(lines []string)
	Clear()

	TurnStart()
	Text(delta string)
	Source(title, url string)
	ResearchRefused(domain string)
	CallsPending()
	ResultsSent()
	TurnDone(b *api.Budget)
	TurnFailed(msg string)
	// Interruptible lets the candidate stop the reply (Esc in the TUI)
	// until stop is called.
	Interruptible(cancel func()) (stop func())

	ToolRead(path string)
	ToolReadDone(path, content string)
	ToolList(dir string)
	ToolListDone(output string)
	ShowEdit(path string, create bool, before, after string)
	ConfirmEdit(path string) bool
	Applied(path, before, after string)
	ConfirmCommand(command, where string, warnings []string) bool
	CommandOutput() io.Writer
	CommandDone(exitCode int, timedOut bool)
	Refused(why string)
}

// Plain is the line-by-line view.
type Plain struct {
	UI    *ui.UI
	Out   io.Writer
	Color bool
}

func (p *Plain) Welcome(w Welcome) {
	fmt.Fprintf(p.Out, "mindfries agent: %s.\n", w.Mode)
	if w.Budget != nil {
		fmt.Fprintf(p.Out, "Budget: %d of %d tokens left.\n", w.Budget.TokensRemaining, w.Budget.TokensTotal)
	}
	fmt.Fprintln(p.Out, "Everything you ask and everything it does is part of the assessment record.")
	fmt.Fprintln(p.Out, "Type a message (end a line with \\ to continue it). /help for commands, /exit to leave.")
}

func (p *Plain) Read() (string, bool) {
	fmt.Fprint(p.Out, "\nyou> ")
	var parts []string
	for {
		line, ok := p.UI.Line()
		if !ok {
			if len(parts) == 0 {
				fmt.Fprintln(p.Out)
			}
			return strings.Join(parts, "\n"), len(parts) > 0
		}
		if strings.HasSuffix(line, "\\") {
			parts = append(parts, strings.TrimSuffix(line, "\\"))
			fmt.Fprint(p.Out, "...> ")
			continue
		}
		parts = append(parts, line)
		return strings.Join(parts, "\n"), true
	}
}

func (p *Plain) Clear()                      {}
func (p *Plain) Notice(msg string)           { fmt.Fprintln(p.Out, msg) }
func (p *Plain) Help(lines []string)         { fmt.Fprintln(p.Out, strings.Join(lines, "\n")) }
func (p *Plain) TurnStart()                  { fmt.Fprint(p.Out, "\nagent> ") }
func (p *Plain) Text(delta string)           { fmt.Fprint(p.Out, delta) }
func (p *Plain) CallsPending()               { fmt.Fprintln(p.Out) }
func (p *Plain) ResultsSent()                { fmt.Fprint(p.Out, "\nagent> ") }
func (p *Plain) TurnFailed(msg string)       { fmt.Fprintf(p.Out, "\n%s\n", msg) }
func (p *Plain) Interruptible(func()) func() { return func() {} }

func (p *Plain) Source(title, url string) {
	fmt.Fprintf(p.Out, "\n  [source] %s (%s)\n", title, url)
}

func (p *Plain) ResearchRefused(domain string) {
	fmt.Fprintf(p.Out, "\n  [research refused: %s isn't an allowed source for this assessment]\n", domain)
}

func (p *Plain) TurnDone(b *api.Budget) {
	if b != nil {
		fmt.Fprintf(p.Out, "\n(%d tokens left)\n", b.TokensRemaining)
		return
	}
	fmt.Fprintln(p.Out)
}

func (p *Plain) ToolRead(path string)              { fmt.Fprintf(p.Out, "  > reading %s\n", path) }
func (p *Plain) ToolReadDone(path, content string) {}
func (p *Plain) ToolList(dir string)               { fmt.Fprintln(p.Out, "  > listing files") }
func (p *Plain) ToolListDone(output string)        {}

func (p *Plain) ShowEdit(path string, create bool, before, after string) {
	fmt.Fprintln(p.Out)
	writeDiff(p.Out, path, before, after, p.Color)
}

func (p *Plain) ConfirmEdit(path string) bool {
	return p.UI.Explicit(fmt.Sprintf("Apply this change to %s?", path))
}

func (p *Plain) Applied(path, before, after string) { fmt.Fprintf(p.Out, "  Applied to %s.\n", path) }

func (p *Plain) ConfirmCommand(command, where string, warnings []string) bool {
	fmt.Fprintf(p.Out, "\nThe assistant wants to run, in %s:\n  %s\n", where, command)
	for _, w := range warnings {
		fmt.Fprintln(p.Out, "  "+w)
	}
	return p.UI.Explicit("Run it?")
}

func (p *Plain) CommandOutput() io.Writer                { return p.Out }
func (p *Plain) CommandDone(exitCode int, timedOut bool) {}
func (p *Plain) Refused(why string)                      { fmt.Fprintf(p.Out, "  (refused: %s)\n", why) }
