package tui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Screen writes the transcript and one "live" region under it: the input
// box, a menu, or the spinner. The live region is redrawn in place; the
// transcript above it only grows.
type Screen struct {
	out   io.Writer
	width func() int

	mu       sync.Mutex
	liveRows int // lines the live region takes
	liveRow  int // the row of the live region the cursor is on

	spin     *spinner
	spinLock sync.Mutex
}

// NewScreen writes to out; width reports the terminal's columns.
func NewScreen(out io.Writer, width func() int) *Screen {
	return &Screen{out: out, width: width}
}

// Width is the usable width: one column short of the terminal, so a full
// line never makes the terminal wrap on its own.
func (s *Screen) Width() int {
	w := 80
	if s.width != nil {
		w = s.width()
	}
	return max(20, w-1)
}

// clearLive erases the live region. s.mu is held.
func (s *Screen) clearLive() {
	if s.liveRows == 0 {
		return
	}
	if s.liveRow > 0 {
		fmt.Fprintf(s.out, "\x1b[%dA", s.liveRow)
	}
	fmt.Fprint(s.out, "\r"+ClearEOS)
	s.liveRows, s.liveRow = 0, 0
}

// Live replaces the live region with lines and puts the cursor at (row,
// col) inside it; row < 0 hides the cursor.
func (s *Screen) Live(lines []string, row, col int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLive()
	fmt.Fprint(s.out, strings.Join(lines, "\r\n"))
	s.liveRows, s.liveRow = len(lines), len(lines)-1
	if row < 0 {
		fmt.Fprint(s.out, HideCursor)
		return
	}
	if up := len(lines) - 1 - row; up > 0 {
		fmt.Fprintf(s.out, "\x1b[%dA", up)
	}
	fmt.Fprint(s.out, "\r")
	if col > 0 {
		fmt.Fprintf(s.out, "\x1b[%dC", col)
	}
	s.liveRow = row
	fmt.Fprint(s.out, ShowCursor)
}

// ClearLive removes the live region.
func (s *Screen) ClearLive() {
	s.mu.Lock()
	s.clearLive()
	s.mu.Unlock()
}

// Print adds text to the transcript, above (in place of) the live region.
// Newlines become "\r\n" so the text lines up in raw mode.
func (s *Screen) Print(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLive()
	fmt.Fprint(s.out, strings.ReplaceAll(text, "\n", "\r\n"))
}

// Println prints lines followed by a new line.
func (s *Screen) Println(lines ...string) {
	s.Print(strings.Join(lines, "\n") + "\n")
}

var spinFrames = []string{"·", "✢", "✳", "✶", "✻", "✽", "✻", "✶", "✳", "✢"}

type spinner struct {
	stop chan struct{}
	done chan struct{}
}

// Spin shows an animated "label… (Ns · hint)" line as the live region until
// the returned stop is called. Starting a new spinner stops the old one.
func (s *Screen) Spin(label, hint string) (stop func()) {
	s.StopSpin()
	sp := &spinner{stop: make(chan struct{}), done: make(chan struct{})}
	s.spinLock.Lock()
	s.spin = sp
	s.spinLock.Unlock()
	start := time.Now()
	go func() {
		defer close(sp.done)
		t := time.NewTicker(120 * time.Millisecond)
		defer t.Stop()
		for i := 0; ; i++ {
			secs := int(time.Since(start).Seconds())
			line := fmt.Sprintf("%s%s %s…%s %s(%ds · %s)%s", Accent, spinFrames[i%len(spinFrames)], label, Reset, Gray, secs, hint, Reset)
			s.Live([]string{line}, -1, 0)
			select {
			case <-sp.stop:
				s.ClearLive()
				return
			case <-t.C:
			}
		}
	}()
	return s.StopSpin
}

// StopSpin stops the spinner, if one is running, and removes its line.
func (s *Screen) StopSpin() {
	s.spinLock.Lock()
	sp := s.spin
	s.spin = nil
	s.spinLock.Unlock()
	if sp == nil {
		return
	}
	close(sp.stop)
	<-sp.done
}
