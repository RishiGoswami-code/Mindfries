package tui

import (
	"strings"
)

// Stream renders the assistant's reply as it arrives: a "⏺" before the
// first line, the rest indented under it, words wrapped to the width, and
// a little Markdown (`code`, **bold**, # headings, - bullets, ``` blocks).
type Stream struct {
	print func(string)
	width int

	started  bool
	col      int
	word     strings.Builder
	wordW    int
	lineHead bool // nothing printed on this line yet
	space    bool // a space waits for the next word
	stars    int  // '*' seen but not yet decided
	code     bool // inside `code`
	bold     bool
	fence    bool // inside a ``` block
	ticks    int  // '`' seen in a row
	head     strings.Builder
}

const indent = "  "

// NewStream prints through print, wrapping at width columns.
func NewStream(print func(string), width int) *Stream {
	return &Stream{print: print, width: max(20, width), lineHead: true}
}

func (s *Stream) style() string {
	st := Reset
	if s.fence || s.code {
		st += Code
	}
	if s.bold {
		st += Bold
	}
	return st
}

func (s *Stream) emit(text string, w int) {
	if !s.started {
		s.print(White + "⏺" + Reset + " " + s.style())
		s.started, s.col = true, 2
	}
	s.print(text)
	s.col += w
	if w > 0 {
		s.lineHead = false
	}
}

func (s *Stream) newline() {
	if !s.started {
		s.emit("", 0)
	}
	s.print(Reset + "\n" + indent + s.style())
	s.col, s.lineHead, s.space = len(indent), true, false
}

func (s *Stream) flushWord() {
	if s.word.Len() == 0 {
		return
	}
	gap := 0
	if s.space {
		gap = 1
	}
	if s.col+gap+s.wordW > s.width && !s.lineHead && !s.fence {
		s.newline()
	} else if s.space {
		s.emit(" ", 1)
	}
	s.space = false
	s.emit(s.word.String(), s.wordW)
	s.word.Reset()
	s.wordW = 0
}

func (s *Stream) flushStars() {
	for s.stars >= 2 {
		s.flushWord()
		s.bold = !s.bold
		s.print(s.style())
		s.stars -= 2
	}
	if s.stars == 1 {
		s.word.WriteRune('*')
		s.wordW++
		s.stars = 0
	}
}

func (s *Stream) flushTicks() {
	switch {
	case s.ticks >= 3:
		s.flushWord()
		s.fence = !s.fence
		s.print(s.style())
	case s.ticks > 0 && !s.fence:
		s.flushWord()
		s.code = !s.code
		s.print(s.style())
	case s.ticks > 0:
		s.word.WriteString(strings.Repeat("`", s.ticks))
		s.wordW += s.ticks
	}
	s.ticks = 0
}

// Write takes the next piece of the reply.
func (s *Stream) Write(text string) {
	for _, r := range text {
		if r != '*' {
			s.flushStars()
		}
		if r != '`' {
			s.flushTicks()
		}
		switch {
		case r == '\r':
		case r == '\n':
			s.flushWord()
			s.code = false
			s.newline()
		case r == '*' && !s.code && !s.fence:
			s.stars++
		case r == '`':
			s.ticks++
		case s.lineHead && !s.fence && s.col <= len(indent)+2 && s.word.Len() == 0 && (r == '#' || ((r == '-' || r == '•') && s.head.Len() == 0)):
			// "# Heading" turns bold; "- item" becomes a bullet.
			if r == '#' {
				s.head.WriteRune(r)
				continue
			}
			s.head.WriteString("•")
		case r == ' ' && s.head.Len() > 0 && s.word.Len() == 0:
			if strings.HasPrefix(s.head.String(), "#") {
				s.bold = true
				s.print(s.style())
			} else {
				s.emit("•", 1)
				s.emit(" ", 1)
			}
			s.head.Reset()
		case r == ' ' || r == '\t':
			s.flushWord()
			if s.fence {
				s.emit(" ", 1)
			} else if !s.lineHead {
				s.space = true
			}
		default:
			if s.head.Len() > 0 {
				s.word.WriteString(s.head.String())
				s.wordW += Width(s.head.String())
				s.head.Reset()
			}
			s.word.WriteRune(r)
			s.wordW += RuneWidth(r)
		}
		if r == '\n' && s.bold && !s.fence {
			// A heading ends with its line.
			s.bold = false
			s.print(s.style())
		}
	}
}

// Close ends the reply with a new line, if anything was written.
func (s *Stream) Close() {
	s.flushStars()
	s.flushTicks()
	s.flushWord()
	if s.started {
		s.print(Reset + "\n")
	}
	s.started = false
}

// Started reports whether any of the reply has been printed.
func (s *Stream) Started() bool { return s.started || s.word.Len() > 0 }
