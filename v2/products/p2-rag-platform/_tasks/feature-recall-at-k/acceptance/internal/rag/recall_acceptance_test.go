package rag

import "testing"

// Hidden acceptance tests for RecallAtK. Applied by the verification harness
// only — never shipped to the candidate.

func recallEngine() *Eval {
	e := NewEngine(40, 10)
	// Two docs are relevant to "alpha"; one is not.
	e.Ingest(Document{ID: "d1", Text: "alpha widget tooling"})
	e.Ingest(Document{ID: "d2", Text: "alpha gadget tooling"})
	e.Ingest(Document{ID: "d3", Text: "zeta gizmo unrelated"})
	return NewEval(e, NewAgent(e, 3))
}

func TestRecallAtKFull(t *testing.T) {
	ev := recallEngine()
	cases := []Case{{Query: "alpha", RelevantDocIDs: []string{"d1", "d2"}}}
	if got := ev.RecallAtK(cases, 3); got != 1.0 {
		t.Fatalf("top-3 should retrieve both relevant docs, recall=%v want 1.0", got)
	}
}

func TestRecallAtKPartial(t *testing.T) {
	ev := recallEngine()
	cases := []Case{{Query: "alpha", RelevantDocIDs: []string{"d1", "d2"}}}
	if got := ev.RecallAtK(cases, 1); got != 0.5 {
		t.Fatalf("top-1 should retrieve one of two relevant docs, recall=%v want 0.5", got)
	}
}
