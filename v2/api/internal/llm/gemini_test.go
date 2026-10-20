package llm

import (
	"strings"
	"testing"
)

func TestBuildPromptCarriesBriefAndJD(t *testing.T) {
	p := buildPrompt("fix `priority.ts`", "Senior React engineer")
	if !strings.Contains(p, "fix `priority.ts`") || !strings.Contains(p, "Senior React engineer") {
		t.Fatal("prompt must embed both the brief and the JD")
	}
	if !strings.Contains(p, "ungradeable") {
		t.Fatal("prompt must carry the hard rules")
	}
}

func TestParseResponse(t *testing.T) {
	ok := []byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello "},{"text":"world"}]}}]}`)
	got, err := parseResponse(ok)
	if err != nil || got != "Hello world" {
		t.Fatalf("parseResponse = %q, %v; want \"Hello world\"", got, err)
	}

	for _, bad := range []string{`{}`, `{"candidates":[]}`, `{"candidates":[{"content":{"parts":[{"text":"   "}]}}]}`, `not json`} {
		if _, err := parseResponse([]byte(bad)); err == nil {
			t.Fatalf("parseResponse(%q) should error", bad)
		}
	}
}

func TestPreservesAnchors(t *testing.T) {
	orig := "Edit `console/src/priority.ts` so `sortTickets` runs `npm test`."
	// Every backticked token survives -> ok.
	if !preservesAnchors(orig, "Tweak console/src/priority.ts: sortTickets, then npm test please") {
		t.Fatal("should accept when all anchors survive")
	}
	// A dropped identifier -> reject (would mislead the candidate vs the tests).
	if preservesAnchors(orig, "Edit the file so sorting works, then run the tests") {
		t.Fatal("should reject when a code anchor is dropped")
	}
}
