package rag

import (
	"strconv"
	"strings"
)

// Tool is a capability the agent can call. Run returns its output and whether
// it handled the input.
type Tool struct {
	Name string
	Run  func(input string) (string, bool)
}

// AnswerResult is what the agent returns: the answer, which tool produced it,
// and the source documents (for retrieval answers).
type AnswerResult struct {
	Answer  string   `json:"answer"`
	Tool    string   `json:"tool"`
	Sources []string `json:"sources"`
}

// Agent answers a question by trying its tools in order, then falling back to
// retrieval over the engine. In production a model would choose the tool and
// synthesize the answer; here the policy is deterministic so the orchestration
// (tool registry, bounded steps, source tracking) is testable on its own.
type Agent struct {
	engine   *Engine
	tools    []Tool
	k        int
	maxSteps int
}

// NewAgent builds an agent over e, retrieving k chunks on fallback. A
// calculator tool is registered by default.
func NewAgent(e *Engine, k int) *Agent {
	if k <= 0 {
		k = 3
	}
	a := &Agent{engine: e, k: k, maxSteps: 4}
	a.Register(Tool{Name: "calc", Run: calc})
	return a
}

// Register adds a tool the agent may call before falling back to retrieval.
func (a *Agent) Register(t Tool) { a.tools = append(a.tools, t) }

// Answer tries each tool (up to maxSteps) and otherwise answers from retrieval.
func (a *Agent) Answer(q string) AnswerResult {
	for i, t := range a.tools {
		if i >= a.maxSteps {
			break
		}
		if out, ok := t.Run(q); ok {
			return AnswerResult{Answer: out, Tool: t.Name}
		}
	}

	hits := a.engine.Query(q, a.k)
	if len(hits) == 0 {
		return AnswerResult{Answer: "I don't have information on that.", Tool: "retrieval"}
	}
	seen := map[string]bool{}
	var sources []string
	for _, h := range hits {
		if !seen[h.Chunk.DocID] {
			seen[h.Chunk.DocID] = true
			sources = append(sources, h.Chunk.DocID)
		}
	}
	return AnswerResult{Answer: hits[0].Chunk.Text, Tool: "retrieval", Sources: sources}
}

// calc handles simple "<int> <op> <int>" arithmetic.
func calc(input string) (string, bool) {
	f := strings.Fields(strings.TrimRight(strings.TrimSpace(input), "?"))
	if len(f) != 3 {
		return "", false
	}
	a, err1 := strconv.Atoi(f[0])
	b, err2 := strconv.Atoi(f[2])
	if err1 != nil || err2 != nil {
		return "", false
	}
	switch f[1] {
	case "+":
		return strconv.Itoa(a + b), true
	case "-":
		return strconv.Itoa(a - b), true
	case "*", "x":
		return strconv.Itoa(a * b), true
	}
	return "", false
}
