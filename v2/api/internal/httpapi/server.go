// Package httpapi is the real V2 engine server: given a role and a job
// description it selects an owned product, resolves a task (JD-cached, variant
// pool), and returns the candidate workspace to set up. It reads the product
// registry from Postgres and the product source from disk, and persists the
// task cache. This is what the CLI / candidate backend talks to — the
// DB-connected replacement for the in-memory fake API.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/delivery"
	"github.com/mindfries/v2/api/internal/engine"
	"github.com/mindfries/v2/api/internal/taskgen"
)

// Registry is the product registry the server reads (implemented by store.DB).
type Registry interface {
	Products(ctx context.Context) ([]engine.Product, error)
}

// Server resolves assessments from the registry + the on-disk product bank.
type Server struct {
	reg          Registry
	productsRoot string // directory holding each product (<root>/<slug>)
	cache        taskgen.Cache
	provider     taskgen.Provider // optional
	poolSize     int

	mu    sync.Mutex
	banks map[string][]taskgen.BankTask // slug -> bank, lazily loaded
}

// New builds the HTTP handler.
func New(reg Registry, productsRoot string, cache taskgen.Cache, provider taskgen.Provider, poolSize int) http.Handler {
	s := &Server{reg: reg, productsRoot: productsRoot, cache: cache, provider: provider, poolSize: poolSize, banks: map[string][]taskgen.BankTask{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v2/products", s.products)
	mux.HandleFunc("POST /v2/assessments/resolve", s.resolve)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	if _, err := s.reg.Products(r.Context()); err != nil {
		status = "degraded: " + err.Error()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (s *Server) products(w http.ResponseWriter, r *http.Request) {
	ps, err := s.reg.Products(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	type pm struct {
		Slug, Name, Domain, GoldenCommit string
		Roles                            []core.Role
	}
	out := make([]pm, 0, len(ps))
	for _, p := range ps {
		var roles []core.Role
		for role := range p.Manifest.Roles {
			roles = append(roles, role)
		}
		out = append(out, pm{p.Slug, p.Name, p.Domain, p.GoldenCommit, roles})
	}
	writeJSON(w, http.StatusOK, out)
}

type resolveReq struct {
	Role core.Role `json:"role"`
	JD   string    `json:"jd"`
}

type resolveResp struct {
	Product struct {
		Slug         string `json:"slug"`
		Name         string `json:"name"`
		GoldenCommit string `json:"golden_commit"`
		Why          string `json:"why"`
	} `json:"product"`
	Task struct {
		TaskVersion string    `json:"task_version"`
		Title       string    `json:"title"`
		Kind        core.Kind `json:"kind"`
		VariantNo   int       `json:"variant_no"`
	} `json:"task"`
	Workspace delivery.Workspace `json:"workspace"`
}

func (s *Server) resolve(w http.ResponseWriter, r *http.Request) {
	var req resolveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if !core.ValidRole(req.Role) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown role"})
		return
	}

	products, err := s.reg.Products(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	choice, err := engine.Select(req.Role, req.JD, products)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	slug := choice.Product.Slug
	productDir := filepath.Join(s.productsRoot, slug)

	bank, err := s.bankFor(slug, productDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	gen := taskgen.New(bank, s.cache, s.provider, s.poolSize)
	rt, err := gen.Resolve(r.Context(), slug, req.Role, req.JD)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	ws, err := delivery.FromResolved(productDir, rt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var resp resolveResp
	resp.Product.Slug = slug
	resp.Product.Name = choice.Product.Name
	resp.Product.GoldenCommit = choice.Product.GoldenCommit
	resp.Product.Why = choice.Why
	resp.Task.TaskVersion = rt.TaskVersion
	resp.Task.Title = titleFor(bank, rt)
	resp.Task.Kind = rt.Kind
	resp.Task.VariantNo = rt.VariantNo
	resp.Workspace = ws
	writeJSON(w, http.StatusOK, resp)
}

// bankFor lazily loads and caches a product's reference task bank.
func (s *Server) bankFor(slug, productDir string) ([]taskgen.BankTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.banks[slug]; ok {
		return b, nil
	}
	b, err := taskgen.LoadBank(productDir, slug)
	if err != nil {
		return nil, err
	}
	s.banks[slug] = b
	return b, nil
}

// titleFor fills the task title from the bank (the cache doesn't persist it).
func titleFor(bank []taskgen.BankTask, rt taskgen.ResolvedTask) string {
	for _, bt := range bank {
		if bt.ID == rt.BankTaskID {
			return bt.Title
		}
	}
	return rt.Title
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
