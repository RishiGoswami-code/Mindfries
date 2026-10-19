// Package term puts the candidate's terminal into raw mode for
// `mindfries agent`'s input box, and reports its size. Standard library
// only: the console API on Windows, termios on Linux and macOS. Anywhere
// else, or when the input isn't a terminal, the agent falls back to plain
// line-by-line input.
package term

import (
	"errors"
	"os"
)

// ErrUnsupported means raw mode isn't available here.
var ErrUnsupported = errors.New("raw terminal mode isn't supported here")

// IsTerminal reports whether f is an interactive terminal.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return isTerminal(f.Fd())
}

// MakeRaw switches in to raw mode (keys arrive one at a time, unechoed,
// Ctrl+C as a byte) and out to understand ANSI escape sequences. Output
// keeps turning "\n" into a new line. restore puts both back.
func MakeRaw(in, out *os.File) (restore func(), err error) {
	if !IsTerminal(in) || !IsTerminal(out) {
		return nil, ErrUnsupported
	}
	undoOut, err := enableANSI(out.Fd())
	if err != nil {
		return nil, err
	}
	undoIn, err := makeRaw(in.Fd())
	if err != nil {
		undoOut()
		return nil, err
	}
	return func() { undoIn(); undoOut() }, nil
}

// EnableANSI makes out understand ANSI colours without changing input. On
// Windows this switches on virtual terminal processing; elsewhere it's a
// no-op. restore puts it back.
func EnableANSI(out *os.File) (restore func(), err error) {
	if !IsTerminal(out) {
		return nil, ErrUnsupported
	}
	return enableANSI(out.Fd())
}

// Width is the terminal's width in columns, or def if it can't be told.
func Width(out *os.File, def int) int {
	if out == nil {
		return def
	}
	w, err := width(out.Fd())
	if err != nil || w <= 0 {
		return def
	}
	return w
}
