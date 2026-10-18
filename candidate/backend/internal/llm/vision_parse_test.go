package llm

import "testing"

func TestParseNumberedCaptions(t *testing.T) {
	// A clean, in-order reply.
	got := parseNumberedCaptions("1: the editor is open\n2: ChatGPT is visible\n3: OK", 3)
	want := []string{"the editor is open", "ChatGPT is visible", "OK"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("caption %d = %q, want %q", i, got[i], want[i])
		}
	}

	// A caption that wraps onto a second, un-numbered line is attached to it.
	got = parseNumberedCaptions("1: the API docs are open in a\nbrowser tab\n2: a phone is in view", 2)
	if got[0] != "the API docs are open in a browser tab" {
		t.Errorf("wrapped caption = %q", got[0])
	}
	if got[1] != "a phone is in view" {
		t.Errorf("caption 1 = %q", got[1])
	}

	// Fewer lines than images: the missing one stays empty, no panic.
	got = parseNumberedCaptions("2: only the second was described", 3)
	if got[0] != "" || got[2] != "" {
		t.Errorf("missing captions should be empty, got %v", got)
	}
	if got[1] != "only the second was described" {
		t.Errorf("caption 1 = %q", got[1])
	}

	// An out-of-range number is ignored rather than indexing out of bounds.
	got = parseNumberedCaptions("5: out of range\n1: in range", 2)
	if got[0] != "in range" {
		t.Errorf("caption 0 = %q", got[0])
	}
}
