package tui

import (
	"strings"
	"unicode"
)

// Input is the message being typed: a buffer, a cursor, and the history of
// messages sent before.
type Input struct {
	buf     []rune
	cur     int
	history [][]rune
	hist    int // index into history while browsing; len(history) = the draft
	draft   []rune
}

// Value is the text typed so far.
func (in *Input) Value() string { return string(in.buf) }

// Empty reports whether nothing is typed.
func (in *Input) Empty() bool { return len(in.buf) == 0 }

// Set replaces the text and puts the cursor at the end.
func (in *Input) Set(s string) {
	in.buf = []rune(s)
	in.cur = len(in.buf)
}

// Submit returns the text, adds it to the history and clears the box.
func (in *Input) Submit() string {
	s := string(in.buf)
	if strings.TrimSpace(s) != "" {
		in.history = append(in.history, append([]rune(nil), in.buf...))
	}
	in.buf, in.cur, in.hist, in.draft = nil, 0, len(in.history), nil
	return s
}

func (in *Input) insert(rs []rune) {
	out := make([]rune, 0, len(in.buf)+len(rs))
	out = append(out, in.buf[:in.cur]...)
	out = append(out, rs...)
	in.buf = append(out, in.buf[in.cur:]...)
	in.cur += len(rs)
}

// Edit applies an editing key. It reports false for keys it doesn't use
// (Enter, Esc, Ctrl+C, Ctrl+D, Tab, Ctrl+L, EOF), which the caller handles.
func (in *Input) Edit(k Key) bool {
	switch k.Kind {
	case KeyRune:
		in.insert([]rune{k.Rune})
	case KeyPaste:
		in.insert([]rune(k.Text))
	case KeyNewline:
		in.insert([]rune{'\n'})
	case KeyBackspace:
		if in.cur > 0 {
			in.buf = append(in.buf[:in.cur-1], in.buf[in.cur:]...)
			in.cur--
		}
	case KeyDelete:
		if in.cur < len(in.buf) {
			in.buf = append(in.buf[:in.cur], in.buf[in.cur+1:]...)
		}
	case KeyLeft:
		if in.cur > 0 {
			in.cur--
		}
	case KeyRight:
		if in.cur < len(in.buf) {
			in.cur++
		}
	case KeyHome:
		in.cur = in.lineStart()
	case KeyEnd:
		in.cur = in.lineEnd()
	case KeyCtrlK:
		in.buf = append(in.buf[:in.cur], in.buf[in.lineEnd():]...)
	case KeyCtrlU:
		s := in.lineStart()
		in.buf = append(in.buf[:s], in.buf[in.cur:]...)
		in.cur = s
	case KeyCtrlW:
		s := in.cur
		for s > 0 && unicode.IsSpace(in.buf[s-1]) {
			s--
		}
		for s > 0 && !unicode.IsSpace(in.buf[s-1]) {
			s--
		}
		in.buf = append(in.buf[:s], in.buf[in.cur:]...)
		in.cur = s
	case KeyUp:
		if !in.moveLine(-1) {
			in.browse(-1)
		}
	case KeyDown:
		if !in.moveLine(1) {
			in.browse(1)
		}
	default:
		return false
	}
	return true
}

func (in *Input) lineStart() int {
	i := in.cur
	for i > 0 && in.buf[i-1] != '\n' {
		i--
	}
	return i
}

func (in *Input) lineEnd() int {
	i := in.cur
	for i < len(in.buf) && in.buf[i] != '\n' {
		i++
	}
	return i
}

// moveLine moves the cursor up or down a line of a multi-line message.
func (in *Input) moveLine(d int) bool {
	start := in.lineStart()
	col := in.cur - start
	if d < 0 {
		if start == 0 {
			return false
		}
		prevEnd := start - 1
		prevStart := prevEnd
		for prevStart > 0 && in.buf[prevStart-1] != '\n' {
			prevStart--
		}
		in.cur = min(prevStart+col, prevEnd)
		return true
	}
	end := in.lineEnd()
	if end == len(in.buf) {
		return false
	}
	nextStart := end + 1
	nextEnd := nextStart
	for nextEnd < len(in.buf) && in.buf[nextEnd] != '\n' {
		nextEnd++
	}
	in.cur = min(nextStart+col, nextEnd)
	return true
}

// browse steps through the history.
func (in *Input) browse(d int) {
	if len(in.history) == 0 {
		return
	}
	if in.hist == len(in.history) {
		in.draft = append([]rune(nil), in.buf...)
	}
	in.hist = max(0, min(len(in.history), in.hist+d))
	if in.hist == len(in.history) {
		in.buf = append([]rune(nil), in.draft...)
	} else {
		in.buf = append([]rune(nil), in.history[in.hist]...)
	}
	in.cur = len(in.buf)
}

// Render lays the input out in a box w columns wide. It returns the lines
// and where the cursor goes (row from the top line, column).
func (in *Input) Render(w int, placeholder string) (lines []string, row, col int) {
	const prompt = "> "
	inner := w - 4 - len(prompt)
	if inner < 4 {
		inner = 4
	}
	var rows [][]rune
	var cur []rune
	curW := 0
	row, col = -1, 0
	place := func() {
		row, col = len(rows), curW
	}
	for i, r := range in.buf {
		if i == in.cur {
			place()
		}
		if r == '\n' {
			rows = append(rows, cur)
			cur, curW = nil, 0
			continue
		}
		rw := RuneWidth(r)
		if curW+rw > inner {
			rows = append(rows, cur)
			cur, curW = nil, 0
			if i == in.cur {
				place()
			}
		}
		cur = append(cur, r)
		curW += rw
	}
	if row < 0 {
		if curW >= inner {
			rows = append(rows, cur)
			cur, curW = nil, 0
		}
		place()
	}
	rows = append(rows, cur)

	body := make([]string, len(rows))
	for i, r := range rows {
		p := "  "
		if i == 0 {
			p = Bold + prompt + Reset
		}
		body[i] = p + string(r)
	}
	if len(in.buf) == 0 && placeholder != "" {
		body[0] = Bold + prompt + Reset + Gray + Truncate(placeholder, inner) + Reset
	}
	lines = Box(body, w, Gray)
	// Inside the box: one border line above, then "│ " and the prompt.
	return lines, row + 1, col + 2 + len(prompt)
}
