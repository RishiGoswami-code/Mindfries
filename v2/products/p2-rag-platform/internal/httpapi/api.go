// Package httpapi exposes the RAG engine, agent, and eval harness over a JSON
// HTTP API. Thin layer: all logic lives in package rag.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mindfries/products/p2-rag-platform/internal/rag"
)

// New returns a handler over an engine, agent, and evaluator.
func New(engine *rag.Engine, agent *rag.Agent, eval *rag.Eval) http.Handler {
	a := &api{engine: engine, agent: agent, eval: eval}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /documents", a.ingest)
	mux.HandleFunc("GET /query", a.query)
	mux.HandleFunc("POST /answer", a.answer)
	mux.HandleFunc("POST /eval", a.runEval)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

type api struct {
	engine *rag.Engine
	agent  *rag.Agent
	eval   *rag.Eval
}

func (a *api) ingest(w http.ResponseWriter, r *http.Request) {
	var d rag.Document
	if !decode(w, r, &d) {
		return
	}
	if d.ID == "" || d.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and text are required"})
		return
	}
	a.engine.Ingest(d)
	writeJSON(w, http.StatusCreated, map[string]int{"chunks": a.engine.Store().Len()})
}

func (a *api) query(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	k := atoiDefault(r.URL.Query().Get("k"), 3)
	writeJSON(w, http.StatusOK, a.engine.Query(q, k))
}

func (a *api) answer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Q string `json:"q"`
	}
	if !decode(w, r, &req) {
		return
	}
	writeJSON(w, http.StatusOK, a.agent.Answer(req.Q))
}

func (a *api) runEval(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cases []rag.Case `json:"cases"`
		K     int        `json:"k"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.K <= 0 {
		req.K = 3
	}
	writeJSON(w, http.StatusOK, map[string]float64{
		"hit_at_k":     a.eval.HitAtK(req.Cases, req.K),
		"answer_match": a.eval.AnswerMatch(req.Cases),
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
