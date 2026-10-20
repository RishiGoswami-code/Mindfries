package rag

import "strings"

// Case is one evaluation example: a query, the documents that are actually
// relevant to it, and a substring the answer should contain.
type Case struct {
	Query          string   `json:"query"`
	RelevantDocIDs []string `json:"relevant_doc_ids"`
	WantSubstr     string   `json:"want_substr"`
}

// Eval runs the retrieval/answer quality harness — the ML-research surface of
// the product. Metrics are kept separate (retrieval vs answer) so a weak link
// is visible rather than hidden in one number.
type Eval struct {
	engine *Engine
	agent  *Agent
}

// NewEval builds an evaluator over an engine and agent.
func NewEval(e *Engine, a *Agent) *Eval { return &Eval{engine: e, agent: a} }

// HitAtK is the fraction of cases where at least one relevant document appears
// in the top-k retrieved chunks.
func (ev *Eval) HitAtK(cases []Case, k int) float64 {
	if len(cases) == 0 {
		return 0
	}
	hits := 0
	for _, c := range cases {
		got := retrievedDocs(ev.engine.Query(c.Query, k))
		if intersects(got, c.RelevantDocIDs) {
			hits++
		}
	}
	return float64(hits) / float64(len(cases))
}

// AnswerMatch is the fraction of cases (that specify a wanted substring) whose
// agent answer contains it, case-insensitively.
func (ev *Eval) AnswerMatch(cases []Case) float64 {
	var n, ok int
	for _, c := range cases {
		if c.WantSubstr == "" {
			continue
		}
		n++
		ans := ev.agent.Answer(c.Query).Answer
		if strings.Contains(strings.ToLower(ans), strings.ToLower(c.WantSubstr)) {
			ok++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(ok) / float64(n)
}

func retrievedDocs(hits []Hit) map[string]bool {
	m := map[string]bool{}
	for _, h := range hits {
		m[h.Chunk.DocID] = true
	}
	return m
}

func intersects(got map[string]bool, want []string) bool {
	for _, w := range want {
		if got[w] {
			return true
		}
	}
	return false
}
