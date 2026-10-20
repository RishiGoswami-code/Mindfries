package rag

import "testing"

func TestChunkOverlap(t *testing.T) {
	text := "a b c d e f g h i j"   // 10 words
	chunks := ChunkText(text, 4, 1) // step 3: [0:4] [3:7] [6:10]
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "a b c d" || chunks[1] != "d e f g" || chunks[2] != "g h i j" {
		t.Fatalf("overlap wrong: %v", chunks)
	}
	// Degenerate params fall back to one chunk.
	if got := ChunkText(text, 0, 0); len(got) != 1 {
		t.Fatalf("bad params should yield one chunk, got %d", len(got))
	}
}

func TestEmbedAndCosine(t *testing.T) {
	a := Embed("password reset login help")
	b := Embed("how to reset my login password")
	c := Embed("quarterly billing invoice export")
	if Cosine(a, b) <= Cosine(a, c) {
		t.Fatalf("overlapping texts should be more similar: ab=%v ac=%v", Cosine(a, b), Cosine(a, c))
	}
	// A unit vector dotted with itself is ~1.
	if s := Cosine(a, a); s < 0.99 || s > 1.01 {
		t.Fatalf("self-similarity should be ~1, got %v", s)
	}
}

func seedEngine() *Engine {
	e := NewEngine(40, 10)
	e.Ingest(Document{ID: "login", Title: "Login help", Text: "To reset your password use the forgot password link on the login page to regain access to your account."})
	e.Ingest(Document{ID: "billing", Title: "Billing", Text: "Download invoices and receipts from the billing page. Refunds are available within thirty days of payment."})
	e.Ingest(Document{ID: "export", Title: "Data export", Text: "Export your workspace data as a CSV file from the admin data export panel at any time."})
	return e
}

func TestRetrievalRanksRelevantDocFirst(t *testing.T) {
	e := seedEngine()
	hits := e.Query("how do I reset my password", 3)
	if len(hits) == 0 || hits[0].Chunk.DocID != "login" {
		t.Fatalf("password query should retrieve the login doc first, got %+v", hits)
	}
}

func TestAgentUsesCalcThenRetrieval(t *testing.T) {
	e := seedEngine()
	a := NewAgent(e, 3)

	if r := a.Answer("12 + 30"); r.Tool != "calc" || r.Answer != "42" {
		t.Fatalf("arithmetic should use calc, got %+v", r)
	}
	r := a.Answer("how do I reset my password")
	if r.Tool != "retrieval" || len(r.Sources) == 0 || r.Sources[0] != "login" {
		t.Fatalf("knowledge query should retrieve with sources, got %+v", r)
	}
}

func evalCases() []Case {
	return []Case{
		{Query: "reset my password", RelevantDocIDs: []string{"login"}, WantSubstr: "password"},
		{Query: "download an invoice", RelevantDocIDs: []string{"billing"}, WantSubstr: "billing"},
		{Query: "export data to csv", RelevantDocIDs: []string{"export"}, WantSubstr: "export"},
	}
}

func TestEvalMetrics(t *testing.T) {
	e := seedEngine()
	ev := NewEval(e, NewAgent(e, 3))
	if h := ev.HitAtK(evalCases(), 1); h < 1.0 {
		t.Fatalf("each case's relevant doc should rank first, HitAt1=%v", h)
	}
	if m := ev.AnswerMatch(evalCases()); m < 1.0 {
		t.Fatalf("answers should contain the wanted substrings, AnswerMatch=%v", m)
	}
}
