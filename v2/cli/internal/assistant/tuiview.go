package assistant

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/tui"
)

// TUI is the rich terminal interface, like an AI coding CLI's: a welcome
// box, an input box, a spinner while the agent works, "⏺" blocks for its
// replies and each tool call, coloured diffs, and approval menus. It needs
// the terminal in raw mode (internal/term); In and Out are that terminal.
type TUI struct {
	keys   *tui.Keys
	scr    *tui.Screen
	input  tui.Input
	budget *api.Budget

	stream   *tui.Stream
	lastCtlC time.Time
	cmdOut   *indentWriter
	// Pause and Resume, if set, leave raw mode while a command runs.
	Pause, Resume func()
}

// NewTUI reads keys from in and draws on out, width columns wide.
func NewTUI(in io.Reader, out io.Writer, width func() int) *TUI {
	return &TUI{keys: tui.NewKeys(in), scr: tui.NewScreen(out, width)}
}

func (t *TUI) w() int { return t.scr.Width() }

func fmtTokens(n int) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (t *TUI) Welcome(w Welcome) {
	t.budget = w.Budget
	body := []string{
		tui.Accent + "✻" + tui.Reset + " Welcome to " + tui.Bold + "mindfries agent" + tui.Reset,
		"",
	}
	if w.Title != "" {
		body = append(body, "  "+tui.Bold+w.Title+tui.Reset)
	}
	body = append(body, tui.Gray+"  "+w.Mode+tui.Reset)
	if w.Budget != nil {
		body = append(body, tui.Gray+fmt.Sprintf("  budget: %s of %s tokens", fmtTokens(w.Budget.TokensRemaining), fmtTokens(w.Budget.TokensTotal))+tui.Reset)
	}
	if w.Workspace != "" {
		body = append(body, tui.Gray+"  cwd: "+w.Workspace+tui.Reset)
	}
	bw := 60
	for _, l := range body {
		bw = max(bw, tui.Width(l)+4)
	}
	box := tui.Box(body, min(t.w(), bw), tui.Accent)
	t.scr.Println(box...)
	t.scr.Println("",
		tui.Gray+" Everything you ask and everything it does is part of the assessment record."+tui.Reset,
		tui.Gray+" Ask about the task, the code, or an error. It proposes; nothing changes without your yes."+tui.Reset,
		"")
}

// hint is the line under the input box.
func (t *TUI) hint(extra string) string {
	left := " ? /help for commands · Ctrl+J new line · Ctrl+C twice to exit"
	if extra != "" {
		left = " " + extra
	}
	right := ""
	if t.budget != nil {
		right = fmtTokens(t.budget.TokensRemaining) + " tokens left "
	}
	gap := t.w() - tui.Width(left) - tui.Width(right)
	if gap < 1 {
		return tui.Gray + left + tui.Reset
	}
	return tui.Gray + left + strings.Repeat(" ", gap) + right + tui.Reset
}

// suggestions are the slash commands matching what's typed.
func (t *TUI) suggestions() []string {
	v := t.input.Value()
	if !strings.HasPrefix(v, "/") || strings.ContainsAny(v, " \n") {
		return nil
	}
	var out []string
	for _, c := range Commands {
		if strings.HasPrefix(c[0], v) {
			style := tui.Gray
			if len(out) == 0 {
				style = tui.Accent
			}
			out = append(out, fmt.Sprintf("  %s%-10s %s%s", style, c[0], c[1], tui.Reset))
		}
	}
	return out
}

func (t *TUI) drawInput(extra string) {
	lines, row, col := t.input.Render(t.w(), "Ask mindfries agent about the task, the code, or an error")
	if s := t.suggestions(); len(s) > 0 {
		lines = append(lines, s...)
	} else {
		lines = append(lines, t.hint(extra))
	}
	t.scr.Live(lines, row, col)
}

func (t *TUI) Read() (string, bool) {
	t.scr.Print(tui.PasteOn)
	defer t.scr.Print(tui.PasteOff)
	extra := ""
	for {
		t.drawInput(extra)
		extra = ""
		k := t.keys.Next()
		if t.input.Edit(k) {
			continue
		}
		switch k.Kind {
		case tui.KeyEnter:
			if s := t.suggestions(); len(s) > 0 && !isCommand(t.input.Value()) {
				t.complete()
			}
			msg := t.input.Submit()
			if strings.TrimSpace(msg) == "" {
				continue
			}
			t.scr.ClearLive()
			t.echo(msg)
			return msg, true
		case tui.KeyTab:
			t.complete()
		case tui.KeyEsc:
			t.input.Set("")
		case tui.KeyCtrlC:
			if !t.input.Empty() {
				t.input.Set("")
				continue
			}
			if time.Since(t.lastCtlC) < 2*time.Second {
				t.scr.ClearLive()
				return "", false
			}
			t.lastCtlC = time.Now()
			extra = "Press Ctrl+C again to exit"
		case tui.KeyCtrlD:
			if t.input.Empty() {
				t.scr.ClearLive()
				return "", false
			}
		case tui.KeyCtrlL:
			t.Clear()
		case tui.KeyEOF:
			t.scr.ClearLive()
			return "", false
		}
	}
}

func isCommand(v string) bool {
	for _, c := range Commands {
		if c[0] == v {
			return true
		}
	}
	return false
}

func (t *TUI) complete() {
	v := t.input.Value()
	for _, c := range Commands {
		if strings.HasPrefix(c[0], v) {
			t.input.Set(c[0])
			return
		}
	}
}

// echo puts the candidate's message into the transcript.
func (t *TUI) echo(msg string) {
	lines := strings.Split(msg, "\n")
	for i, l := range lines {
		p := "  "
		if i == 0 {
			p = "> "
		}
		lines[i] = tui.Gray + p + l + tui.Reset
	}
	t.scr.Println(lines...)
	t.scr.Println("")
}

func (t *TUI) Notice(msg string) {
	t.endText()
	t.scr.Println(tui.Gray+"  ⎿  "+strings.ReplaceAll(msg, "\n", "\n     ")+tui.Reset, "")
}

func (t *TUI) Help(lines []string) {
	out := []string{tui.Bold + " Commands" + tui.Reset}
	for _, l := range lines {
		out = append(out, "   "+l)
	}
	out = append(out, "",
		tui.Bold+" Keys"+tui.Reset,
		"   Enter      send",
		"   Ctrl+J     new line (or paste several lines)",
		"   ↑ / ↓      earlier messages",
		"   Esc        stop the agent while it's working; clear the box otherwise",
		"   Ctrl+C     clear the box; twice to exit",
		"")
	t.scr.Println(out...)
}

func (t *TUI) Clear() {
	t.scr.Print("\x1b[2J\x1b[H")
}

func (t *TUI) TurnStart() {
	t.scr.Spin("Thinking", "esc to interrupt")
}

func (t *TUI) Text(delta string) {
	if t.stream == nil {
		t.scr.StopSpin()
		t.stream = tui.NewStream(t.scr.Print, t.w()-2)
	}
	t.stream.Write(delta)
}

// endText closes the reply being streamed, if any.
func (t *TUI) endText() {
	t.scr.StopSpin()
	if t.stream != nil {
		t.stream.Close()
		t.stream = nil
		t.scr.Println("")
	}
}

func (t *TUI) Source(title, url string) {
	t.endText()
	t.scr.Println(tui.Gray+"  ⎿  Source: "+tui.Reset+title+tui.Gray+" ("+url+")"+tui.Reset, "")
}

func (t *TUI) ResearchRefused(domain string) {
	t.endText()
	t.scr.Println(tui.Yellow+"  ⎿  Research refused: "+domain+" isn't an allowed source for this assessment"+tui.Reset, "")
}

func (t *TUI) CallsPending() { t.endText() }

func (t *TUI) ResultsSent() { t.scr.Spin("Working", "esc to interrupt") }

func (t *TUI) TurnDone(b *api.Budget) {
	t.endText()
	if b != nil {
		t.budget = b
	}
}

func (t *TUI) TurnFailed(msg string) {
	t.endText()
	t.scr.Println(tui.Red+"  ⎿  "+msg+tui.Reset, "")
}

// Interruptible watches for Esc or Ctrl+C until stop is called.
func (t *TUI) Interruptible(cancel func()) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			k, ok := t.keys.NextUntil(stop)
			if !ok {
				return
			}
			if k.Kind == tui.KeyEsc || k.Kind == tui.KeyCtrlC {
				cancel()
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(stop)
			<-done
		})
	}
}

// tool starts a "⏺ Name(arg)" block.
func (t *TUI) tool(name, arg string) {
	t.endText()
	t.scr.Println(tui.Green + "⏺" + tui.Reset + " " + tui.Bold + name + tui.Reset + "(" + arg + ")")
}

func (t *TUI) result(lines ...string) {
	for i, l := range lines {
		p := "     "
		if i == 0 {
			p = "  ⎿  "
		}
		lines[i] = tui.Gray + p + tui.Reset + l
	}
	t.scr.Println(lines...)
	t.scr.Println("")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (t *TUI) ToolRead(path string) { t.tool("Read", path) }

func (t *TUI) ToolReadDone(path, content string) {
	t.result(tui.Gray + "Read " + plural(len(lines(content)), "line", "lines") + tui.Reset)
}

func (t *TUI) ToolList(dir string) {
	if dir == "" {
		dir = "."
	}
	t.tool("List", dir)
}

func (t *TUI) ToolListDone(output string) {
	t.result(tui.Gray + "Listed " + plural(len(lines(output)), "file", "files") + tui.Reset)
}

func (t *TUI) ShowEdit(path string, create bool, before, after string) {
	name := "Update"
	if create {
		name = "Create"
	}
	t.tool(name, path)
	t.scr.Println(diffBlock(before, after, t.w())...)
	t.scr.Println("")
}

func (t *TUI) ConfirmEdit(path string) bool {
	return tui.Menu(t.scr, t.keys, fmt.Sprintf("Do you want to make this edit to %s?", path),
		[]string{"Yes", "No, and tell mindfries agent what to do differently"}) == 0
}

func (t *TUI) Applied(path, before, after string) {
	add, del := 0, 0
	for _, o := range diffLines(lines(before), lines(after)) {
		switch o.kind {
		case '+':
			add++
		case '-':
			del++
		}
	}
	t.result(fmt.Sprintf("Updated %s with %s and %s", tui.Bold+path+tui.Reset, plural(add, "addition", "additions"), plural(del, "removal", "removals")))
}

func (t *TUI) ConfirmCommand(command, where string, warnings []string) bool {
	t.tool("Bash", command)
	var body []string
	if where != "the task folder" {
		body = append(body, tui.Gray+"in "+where+tui.Reset)
	}
	for _, w := range warnings {
		body = append(body, tui.Yellow+"⚠ "+w+tui.Reset)
	}
	if len(body) > 0 {
		t.result(body...)
	}
	ok := tui.Menu(t.scr, t.keys, "Do you want to run this command?", []string{"Yes", "No, and tell mindfries agent what to do differently"}) == 0
	if ok && t.Pause != nil {
		t.Pause()
	}
	return ok
}

func (t *TUI) CommandOutput() io.Writer {
	if t.cmdOut == nil || t.cmdOut.done {
		t.cmdOut = &indentWriter{print: t.scr.Print, max: 12}
	}
	return t.cmdOut
}

func (t *TUI) CommandDone(exitCode int, timedOut bool) {
	if t.Resume != nil {
		t.Resume()
	}
	w := t.CommandOutput().(*indentWriter)
	w.finish()
	status := tui.Green + "✓ exit code 0" + tui.Reset
	if exitCode != 0 {
		status = tui.Red + fmt.Sprintf("✗ exit code %d", exitCode) + tui.Reset
	}
	if timedOut {
		status = tui.Red + "✗ timed out" + tui.Reset
	}
	t.scr.Println("     "+status, "")
}

func (t *TUI) Refused(why string) {
	t.endText()
	t.scr.Println(tui.Red+"  ⎿  Refused: "+why+tui.Reset, "")
}

// indentWriter shows a command's output under its "⏺ Bash" block, the
// first max lines of it.
type indentWriter struct {
	print   func(string)
	max     int
	lines   int
	hidden  int
	atStart bool
	started bool
	done    bool
}

func (w *indentWriter) Write(p []byte) (int, error) {
	for _, l := range strings.SplitAfter(string(p), "\n") {
		if l == "" {
			continue
		}
		if !w.started || w.atStart {
			if w.lines >= w.max {
				if strings.HasSuffix(l, "\n") {
					w.hidden++
				}
				continue
			}
			p := "     "
			if !w.started {
				p = "  ⎿  "
			}
			w.print(tui.Gray + p + tui.Reset)
			w.started = true
		}
		w.print(strings.TrimRight(l, "\r\n"))
		w.atStart = strings.HasSuffix(l, "\n")
		if w.atStart {
			w.print("\n")
			w.lines++
		}
	}
	return len(p), nil
}

func (w *indentWriter) finish() {
	if w.started && !w.atStart {
		w.print("\n")
	}
	if w.hidden > 0 {
		w.print(tui.Gray + fmt.Sprintf("     … +%d more lines", w.hidden) + tui.Reset + "\n")
	}
	w.done = true
}

// diffBlock is a diff with line numbers, removals on red and additions on
// green, three lines of context.
func diffBlock(before, after string, width int) []string {
	ops := diffLines(lines(before), lines(after))
	const ctx = 3
	show := make([]bool, len(ops))
	for i, o := range ops {
		if o.kind != ' ' {
			for k := max(0, i-ctx); k <= min(len(ops)-1, i+ctx); k++ {
				show[k] = true
			}
		}
	}
	var out []string
	oldN, newN := 0, 0
	skipped := false
	textW := max(10, width-14)
	for i, o := range ops {
		switch o.kind {
		case ' ':
			oldN++
			newN++
		case '-':
			oldN++
		case '+':
			newN++
		}
		if !show[i] {
			skipped = true
			continue
		}
		if skipped && len(out) > 0 {
			out = append(out, tui.Gray+"       ⋮"+tui.Reset)
		}
		skipped = false
		text := tui.Truncate(strings.ReplaceAll(o.text, "\t", "    "), textW)
		switch o.kind {
		case '-':
			out = append(out, fmt.Sprintf("  %s%5d -%s %s%s", tui.DelBG, oldN, tui.Reset+tui.DelBG, tui.Pad(text, textW), tui.Reset))
		case '+':
			out = append(out, fmt.Sprintf("  %s%5d +%s %s%s", tui.AddBG, newN, tui.Reset+tui.AddBG, tui.Pad(text, textW), tui.Reset))
		default:
			out = append(out, fmt.Sprintf("  %s%5d  %s %s", tui.Gray, newN, tui.Reset, text))
		}
	}
	if len(out) == 0 {
		out = append(out, tui.Gray+"     (no change)"+tui.Reset)
	}
	return out
}
