// Package ui asks the candidate questions on the terminal. With Yes set it
// takes the default answers, except where a question must be answered by a
// person (Explicit).
package ui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// UI reads answers from In and writes questions to Out.
type UI struct {
	in  *bufio.Reader
	Out io.Writer
	// Yes takes the default for every ordinary question.
	Yes bool
}

func New(in io.Reader, out io.Writer, yes bool) *UI {
	return &UI{in: bufio.NewReader(in), Out: out, Yes: yes}
}

// Line reads one line; ok is false at end of input.
func (u *UI) Line() (string, bool) {
	s, err := u.in.ReadString('\n')
	if err != nil && s == "" {
		return "", false
	}
	return strings.TrimRight(s, "\r\n"), true
}

// Ask asks a question with a default answer.
func (u *UI) Ask(q, def string) string {
	if u.Yes {
		fmt.Fprintf(u.Out, "%s %s\n", q, def)
		return def
	}
	if def != "" {
		fmt.Fprintf(u.Out, "%s [%s]: ", q, def)
	} else {
		fmt.Fprintf(u.Out, "%s: ", q)
	}
	s, ok := u.Line()
	if !ok {
		fmt.Fprintln(u.Out)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

// Confirm asks a yes/no question.
func (u *UI) Confirm(q string, def bool) bool {
	if u.Yes {
		fmt.Fprintf(u.Out, "%s %s\n", q, yn(def))
		return def
	}
	return u.ask(q, def)
}

// Explicit asks a yes/no question that Yes does not answer, defaulting to
// no: overwriting files, running commands the candidate hasn't seen.
func (u *UI) Explicit(q string) bool {
	return u.ask(q, false)
}

func (u *UI) ask(q string, def bool) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	for {
		fmt.Fprintf(u.Out, "%s %s ", q, hint)
		s, ok := u.Line()
		if !ok {
			fmt.Fprintln(u.Out)
			return def
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "":
			return def
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

// Choose asks for one of options by number and returns its index.
func (u *UI) Choose(q string, options []string, def int) int {
	fmt.Fprintln(u.Out, q)
	for i, o := range options {
		fmt.Fprintf(u.Out, "  %d. %s\n", i+1, o)
	}
	if u.Yes {
		fmt.Fprintf(u.Out, "Choice: %d\n", def+1)
		return def
	}
	for {
		fmt.Fprintf(u.Out, "Choice [%d]: ", def+1)
		s, ok := u.Line()
		if !ok {
			fmt.Fprintln(u.Out)
			return def
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(options) {
			return n - 1
		}
	}
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
