package tui

import (
	"strings"
	"unicode"
)

// ANSI styles. Everything the interface draws goes through these, so a
// test can strip them (Strip) and read the plain text.
const (
	Reset  = "\x1b[0m"
	Bold   = "\x1b[1m"
	Dim    = "\x1b[2m"
	Italic = "\x1b[3m"
	Gray   = "\x1b[90m"
	Red    = "\x1b[31m"
	Green  = "\x1b[32m"
	Yellow = "\x1b[33m"
	Blue   = "\x1b[34m"
	Cyan   = "\x1b[36m"
	White  = "\x1b[97m"
	// Accent is the mindfries colour, a warm salmon.
	Accent = "\x1b[38;5;209m"
	// Code is inline `code`.
	Code = "\x1b[38;5;153m"
	// AddBG and DelBG tint diff lines.
	AddBG = "\x1b[48;5;22m"
	DelBG = "\x1b[48;5;52m"

	HideCursor = "\x1b[?25l"
	ShowCursor = "\x1b[?25h"
	// PasteOn and PasteOff switch bracketed paste, so a pasted block
	// arrives as one KeyPaste instead of many Enters.
	PasteOn  = "\x1b[?2004h"
	PasteOff = "\x1b[?2004l"
	ClearEOL = "\x1b[K"
	ClearEOS = "\x1b[J"
)

// Strip removes ANSI escape sequences.
func Strip(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		i++
		if i < len(s) && s[i] == '[' {
			for i++; i < len(s) && !(s[i] >= 0x40 && s[i] <= 0x7e); i++ {
			}
		}
	}
	return b.String()
}

// RuneWidth is how many columns r takes: 2 for wide East Asian characters
// and most emoji, 0 for combining marks, 1 otherwise.
func RuneWidth(r rune) int {
	switch {
	case r == 0, unicode.Is(unicode.Mn, r):
		return 0
	case r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1f64f) ||
		(r >= 0x1f900 && r <= 0x1f9ff) ||
		(r >= 0x20000 && r <= 0x3fffd)):
		return 2
	}
	return 1
}

// Width is the columns s takes on screen, ignoring ANSI sequences.
func Width(s string) int {
	n := 0
	for _, r := range Strip(s) {
		n += RuneWidth(r)
	}
	return n
}

// Pad right-pads s with spaces to w columns.
func Pad(s string, w int) string {
	if d := w - Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// Truncate cuts plain text s to at most w columns, ending in "…" if cut.
func Truncate(s string, w int) string {
	if Width(s) <= w {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		rw := RuneWidth(r)
		if n+rw > w-1 {
			break
		}
		b.WriteRune(r)
		n += rw
	}
	return b.String() + "…"
}

// Box draws lines inside a rounded border w columns wide, in borderStyle.
func Box(lines []string, w int, borderStyle string) []string {
	inner := w - 4
	out := []string{borderStyle + "╭" + strings.Repeat("─", w-2) + "╮" + Reset}
	for _, l := range lines {
		if Width(l) > inner {
			l = Truncate(Strip(l), inner)
		}
		out = append(out, borderStyle+"│"+Reset+" "+Pad(l, inner)+" "+borderStyle+"│"+Reset)
	}
	return append(out, borderStyle+"╰"+strings.Repeat("─", w-2)+"╯"+Reset)
}
