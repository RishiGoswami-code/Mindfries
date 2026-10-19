package tui

import (
	"bytes"
	"strings"
	"testing"
)

func keysFrom(s string) *Keys { return NewKeys(strings.NewReader(s)) }

func TestKeysParsesSequences(t *testing.T) {
	k := keysFrom("aé\x1b[A\x1b[B\x1b[C\x1b[D\x1b[3~\x1bOH\x1b[F\x7f\r\n\x03\x04\x1b[200~line1\r\nline2\x1b[201~\x1b")
	want := []KeyKind{KeyRune, KeyRune, KeyUp, KeyDown, KeyRight, KeyLeft, KeyDelete, KeyHome, KeyEnd, KeyBackspace, KeyEnter, KeyNewline, KeyCtrlC, KeyCtrlD, KeyPaste, KeyEsc, KeyEOF}
	for i, w := range want {
		got := k.Next()
		if got.Kind != w {
			t.Fatalf("key %d: got %v, want %v", i, got.Kind, w)
		}
		if i == 1 && got.Rune != 'é' {
			t.Fatalf("utf-8: %q", got.Rune)
		}
		if got.Kind == KeyPaste && got.Text != "line1\nline2" {
			t.Fatalf("paste: %q", got.Text)
		}
	}
}

func TestNextUntilGivesUp(t *testing.T) {
	r, w := ioPipe()
	k := NewKeys(r)
	stop := make(chan struct{})
	close(stop)
	if _, ok := k.NextUntil(stop); ok {
		t.Fatal("NextUntil returned a key with nothing typed")
	}
	w.Write([]byte("x"))
	if key := k.Next(); key.Kind != KeyRune || key.Rune != 'x' {
		t.Fatalf("after giving up: %+v", key)
	}
}

func TestInputEditing(t *testing.T) {
	var in Input
	for _, k := range []Key{{Kind: KeyRune, Rune: 'h'}, {Kind: KeyRune, Rune: 'i'}, {Kind: KeyLeft}, {Kind: KeyRune, Rune: 'X'}, {Kind: KeyEnd}, {Kind: KeyNewline}, {Kind: KeyPaste, Text: "two words"}, {Kind: KeyCtrlW}} {
		in.Edit(k)
	}
	if got := in.Value(); got != "hXi\ntwo " {
		t.Fatalf("value = %q", got)
	}
	if got := in.Submit(); got != "hXi\ntwo " || !in.Empty() {
		t.Fatalf("submit = %q", got)
	}
	in.Edit(Key{Kind: KeyUp})
	if in.Value() != "hXi\ntwo " {
		t.Fatalf("history: %q", in.Value())
	}
	in.Edit(Key{Kind: KeyDown})
	if !in.Empty() {
		t.Fatalf("back to the draft: %q", in.Value())
	}
}

func TestInputRenderWrapsAndPlacesTheCursor(t *testing.T) {
	var in Input
	in.Set(strings.Repeat("a", 30))
	lines, row, col := in.Render(20, "")
	// 20 wide: 14 columns of text per row, so 30 runes take 3 rows.
	if len(lines) != 5 {
		t.Fatalf("lines:\n%s", strings.Join(lines, "\n"))
	}
	for _, l := range lines {
		if Width(l) != 20 {
			t.Fatalf("line %q is %d wide", Strip(l), Width(l))
		}
	}
	if row != 3 || col != 4+2 {
		t.Fatalf("cursor at %d,%d", row, col)
	}
	in.Set("")
	lines, _, _ = in.Render(40, "Ask anything")
	if !strings.Contains(Strip(lines[1]), "> Ask anything") {
		t.Fatalf("placeholder: %q", Strip(lines[1]))
	}
}

func render(text string, width int) string {
	var b bytes.Buffer
	s := NewStream(func(p string) { b.WriteString(p) }, width)
	s.Write(text)
	s.Close()
	return Strip(b.String())
}

func TestStreamMarkdown(t *testing.T) {
	got := render("# Plan\nUse **a dict** and `time.time()`.\n- first\n- second", 80)
	want := "⏺ Plan\n  Use a dict and time.time().\n  • first\n  • second\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

func TestStreamWrapsWords(t *testing.T) {
	got := render("one two three four five six seven", 22)
	want := "⏺ one two three four\n  five six seven\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// The same text in small pieces renders the same.
	var b bytes.Buffer
	s := NewStream(func(p string) { b.WriteString(p) }, 22)
	for _, p := range []string{"one t", "wo thr", "ee four", " five", " six", " sev", "en"} {
		s.Write(p)
	}
	s.Close()
	if Strip(b.String()) != want {
		t.Fatalf("in pieces: %q", Strip(b.String()))
	}
}

func TestMenu(t *testing.T) {
	var out bytes.Buffer
	scr := NewScreen(&out, func() int { return 60 })
	if i := Menu(scr, keysFrom("\x1b[B\r"), "Run it?", []string{"Yes", "No"}); i != 1 {
		t.Fatalf("down, enter: %d", i)
	}
	if i := Menu(scr, keysFrom("y"), "Run it?", []string{"Yes", "No"}); i != 0 {
		t.Fatalf("y: %d", i)
	}
	if i := Menu(scr, keysFrom("\x1b"), "Run it?", []string{"Yes", "No"}); i != 1 {
		t.Fatalf("esc: %d", i)
	}
	if !strings.Contains(Strip(out.String()), "❯ 1. Yes") {
		t.Fatalf("menu not drawn:\n%s", Strip(out.String()))
	}
}

func TestBoxWidth(t *testing.T) {
	for _, l := range Box([]string{"short", strings.Repeat("long ", 30)}, 30, Gray) {
		if Width(l) != 30 {
			t.Fatalf("%q is %d wide", Strip(l), Width(l))
		}
	}
}
