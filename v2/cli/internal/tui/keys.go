// Package tui is the terminal interface of `mindfries agent`: keys read in
// raw mode, an input box, a spinner, and the transcript styling, in the
// manner of an AI coding CLI. It only reads an io.Reader and writes
// an io.Writer, so tests drive it without a real console; internal/term
// puts the console into raw mode.
package tui

import (
	"io"
	"time"
	"unicode/utf8"
)

// KeyKind says what a key is.
type KeyKind int

const (
	KeyRune KeyKind = iota
	KeyEnter
	KeyNewline // Ctrl+J or Alt+Enter: a new line inside the message
	KeyBackspace
	KeyDelete
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyTab
	KeyEsc
	KeyCtrlC
	KeyCtrlD
	KeyCtrlK // delete to the end of the line
	KeyCtrlU // delete to the start of the line
	KeyCtrlW // delete the word before the cursor
	KeyCtrlL // redraw
	KeyPaste // Text holds what was pasted
	KeyEOF   // the input ended
)

// Key is one keypress, or one paste.
type Key struct {
	Kind KeyKind
	Rune rune
	Text string
}

// escWait is how long a lone Esc waits for the rest of a sequence.
var escWait = 30 * time.Millisecond

// Keys turns raw terminal input into keys. One goroutine reads the input;
// Next takes keys in order.
type Keys struct {
	bytes   chan []byte
	pending []byte
	eof     bool
}

// NewKeys starts reading r.
func NewKeys(r io.Reader) *Keys {
	k := &Keys{bytes: make(chan []byte, 64)}
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				k.bytes <- append([]byte(nil), buf[:n]...)
			}
			if err != nil {
				close(k.bytes)
				return
			}
		}
	}()
	return k
}

// byteWithin returns the next byte, waiting at most wait (forever if wait
// is negative).
func (k *Keys) byteWithin(wait time.Duration) (byte, bool) {
	if len(k.pending) == 0 {
		if k.eof {
			return 0, false
		}
		var chunk []byte
		var ok bool
		if wait < 0 {
			chunk, ok = <-k.bytes
		} else {
			select {
			case chunk, ok = <-k.bytes:
			case <-time.After(wait):
				return 0, false
			}
		}
		if !ok {
			k.eof = true
			return 0, false
		}
		k.pending = chunk
	}
	b := k.pending[0]
	k.pending = k.pending[1:]
	return b, true
}

// Next blocks until the next key.
func (k *Keys) Next() Key {
	key, _ := k.NextUntil(nil)
	return key
}

// NextUntil is Next, giving up (ok false) when stop is closed before a key
// begins. A key already begun is read to its end.
func (k *Keys) NextUntil(stop <-chan struct{}) (key Key, ok bool) {
	if len(k.pending) == 0 && !k.eof && stop != nil {
		select {
		case chunk, open := <-k.bytes:
			if !open {
				k.eof = true
			} else {
				k.pending = chunk
			}
		case <-stop:
			return Key{}, false
		}
	}
	return k.next(), true
}

func (k *Keys) next() Key {
	b, ok := k.byteWithin(-1)
	if !ok {
		return Key{Kind: KeyEOF}
	}
	switch b {
	case 0x1b:
		return k.escape()
	case '\r':
		return Key{Kind: KeyEnter}
	case '\n':
		return Key{Kind: KeyNewline}
	case 0x7f, 0x08:
		return Key{Kind: KeyBackspace}
	case '\t':
		return Key{Kind: KeyTab}
	case 0x03:
		return Key{Kind: KeyCtrlC}
	case 0x04:
		return Key{Kind: KeyCtrlD}
	case 0x01:
		return Key{Kind: KeyHome}
	case 0x05:
		return Key{Kind: KeyEnd}
	case 0x02:
		return Key{Kind: KeyLeft}
	case 0x06:
		return Key{Kind: KeyRight}
	case 0x0b:
		return Key{Kind: KeyCtrlK}
	case 0x15:
		return Key{Kind: KeyCtrlU}
	case 0x17:
		return Key{Kind: KeyCtrlW}
	case 0x0c:
		return Key{Kind: KeyCtrlL}
	}
	if b < 0x20 {
		return k.next() // another control key: ignore it
	}
	return Key{Kind: KeyRune, Rune: k.decode(b)}
}

// decode reads the rest of a UTF-8 sequence that starts with b.
func (k *Keys) decode(b byte) rune {
	if b < utf8.RuneSelf {
		return rune(b)
	}
	n := 0
	switch {
	case b&0xE0 == 0xC0:
		n = 1
	case b&0xF0 == 0xE0:
		n = 2
	case b&0xF8 == 0xF0:
		n = 3
	}
	seq := []byte{b}
	for i := 0; i < n; i++ {
		c, ok := k.byteWithin(escWait)
		if !ok {
			break
		}
		seq = append(seq, c)
	}
	r, _ := utf8.DecodeRune(seq)
	return r
}

func (k *Keys) escape() Key {
	b, ok := k.byteWithin(escWait)
	if !ok {
		return Key{Kind: KeyEsc}
	}
	switch b {
	case '[':
		return k.csi()
	case 'O':
		c, _ := k.byteWithin(escWait)
		return arrow(c)
	case '\r':
		return Key{Kind: KeyNewline}
	case 0x7f:
		return Key{Kind: KeyCtrlW}
	}
	// Alt+key: take the key as typed.
	return Key{Kind: KeyRune, Rune: k.decode(b)}
}

func arrow(c byte) Key {
	switch c {
	case 'A':
		return Key{Kind: KeyUp}
	case 'B':
		return Key{Kind: KeyDown}
	case 'C':
		return Key{Kind: KeyRight}
	case 'D':
		return Key{Kind: KeyLeft}
	case 'H':
		return Key{Kind: KeyHome}
	case 'F':
		return Key{Kind: KeyEnd}
	}
	return Key{Kind: KeyEsc}
}

// csi reads a control sequence: parameters, then a final byte.
func (k *Keys) csi() Key {
	var params []byte
	for {
		c, ok := k.byteWithin(escWait)
		if !ok {
			return Key{Kind: KeyEsc}
		}
		if c >= 0x40 && c <= 0x7e {
			if c != '~' {
				return arrow(c)
			}
			break
		}
		params = append(params, c)
	}
	switch string(params) {
	case "3":
		return Key{Kind: KeyDelete}
	case "1", "7":
		return Key{Kind: KeyHome}
	case "4", "8":
		return Key{Kind: KeyEnd}
	case "200":
		return k.paste()
	}
	return k.next() // a sequence we don't use
}

// paste collects a bracketed paste up to ESC [ 201 ~.
func (k *Keys) paste() Key {
	const end = "\x1b[201~"
	var text []byte
	for {
		c, ok := k.byteWithin(time.Second)
		if !ok {
			break
		}
		text = append(text, c)
		if len(text) >= len(end) && string(text[len(text)-len(end):]) == end {
			text = text[:len(text)-len(end)]
			break
		}
	}
	return Key{Kind: KeyPaste, Text: pasteText(text)}
}
