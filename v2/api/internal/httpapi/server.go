// Package httpapi is the real V2 engine server: given a role and a job
// description it selects an owned product, resolves a task (JD-cached, variant
// pool), and returns the candidate workspace to set up. It reads the product
// registry from Postgres and the product source from disk, and persists the
// task cache. This is what the CLI / candidate backend talks to — the
// DB-connected replacement for the in-memory fake API.
package httpapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/assistant"
	"github.com/mindfries/v2/api/internal/blob"
	"github.com/mindfries/v2/api/internal/config"
	"github.com/mindfries/v2/api/internal/mw"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/delivery"
	"github.com/mindfries/v2/api/internal/engine"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/taskgen"
)

// Registry is the product registry the server reads (implemented by store.DB).
type Registry interface {
	Products(ctx context.Context) ([]engine.Product, error)
}

// Options is everything the server is built from. Anything left nil turns
// its routes off (a server with no GitHub client has no /github routes), so a
// partial deployment says "not configured" plainly instead of failing later.
type Options struct {
	Config     config.Config
	Registry   Registry
	Cache      taskgen.Cache
	Provider   taskgen.Provider // optional: model-tailored briefs
	Sessions   session.Store    // enables the CLI contract
	Devices    DeviceStore
	Configs    ConfigStore
	GitHub     GitHubStore
	Monitor    MonitorStore
	Activity   ActivityStore
	Panel      PanelStore
	Assistant  AssistantStore
	Avatar     AvatarStore
	Staff      StaffStore
	Privacy    PrivacyStore
	Compliance ComplianceStore
	Model      assistant.Model       // nil: Gemini if a key is set, else a scripted model
	Researcher *assistant.Researcher // nil: the assessment's own research list
	Blobs      blob.Store
	Hub        *activity.Hub
	Log        *slog.Logger
	Now        func() time.Time
}

// Server resolves assessments from the registry + the on-disk product bank,
// serves the CLI session contract, and the portals' and panel's routes.
type Server struct {
	cfg          config.Config
	reg          Registry
	productsRoot string // directory holding each product (<root>/<slug>)
	cache        taskgen.Cache
	provider     taskgen.Provider // optional
	poolSize     int
	sessions     session.Store // optional; enables the /v2/cli contract
	devices      DeviceStore
	configs      ConfigStore
	github       GitHubStore
	monitor      MonitorStore
	acts         ActivityStore
	panelStore   PanelStore
	assist       AssistantStore
	avatar       AvatarStore
	avatarLimit  *mw.Limiter
	staff        StaffStore
	privacy      PrivacyStore
	compliance   ComplianceStore
	assistModel  assistant.Model
	researcher   *assistant.Researcher
	blobs        blob.Store
	hub          *activity.Hub
	signKeyID    string
	signKey      ed25519.PrivateKey // nil -> fall back to the dev key
	log          *slog.Logger
	now          func() time.Time

	mu    sync.Mutex
	banks map[string][]taskgen.BankTask // slug -> bank, lazily loaded
}

// New builds the HTTP handler with the engine and the CLI session contract
// only. sessions may be nil (engine-only). signKey may be nil to sign bundles
// with the dev key (dev/test). It is NewServer with a minimal Options.
func New(reg Registry, productsRoot string, cache taskgen.Cache, provider taskgen.Provider, poolSize int, sessions session.Store, signKeyID string, signKey ed25519.PrivateKey) http.Handler {
	// The few settings the engine-only server reads from the environment (the
	// CLI downloads, the public address, the admin token); the rest comes in
	// through NewServer's Options.
	cfg, _ := config.Load(os.Getenv, false)
	cfg.ProductsRoot, cfg.PoolSize, cfg.SigningKeyID, cfg.SigningKey = productsRoot, poolSize, signKeyID, signKey
	return NewServer(Options{Config: cfg, Registry: reg, Cache: cache, Provider: provider, Sessions: sessions})
}

// NewServer builds the handler from Options, wrapped in the request
// middleware (request IDs, panic recovery, access log).
func NewServer(o Options) http.Handler {
	s := &Server{cfg: o.Config, reg: o.Registry, productsRoot: o.Config.ProductsRoot, cache: o.Cache, provider: o.Provider,
		poolSize: o.Config.PoolSize, sessions: o.Sessions, devices: o.Devices, configs: o.Configs, github: o.GitHub, monitor: o.Monitor, acts: o.Activity, panelStore: o.Panel, assist: o.Assistant, avatar: o.Avatar, staff: o.Staff, privacy: o.Privacy, compliance: o.Compliance, assistModel: o.Model, researcher: o.Researcher, blobs: o.Blobs, hub: o.Hub,
		signKeyID: o.Config.SigningKeyID, signKey: o.Config.SigningKey, log: o.Log, now: o.Now,
		banks: map[string][]taskgen.BankTask{}}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.hub == nil {
		s.hub = activity.NewHub()
	}
	if s.now == nil {
		s.now = func() time.Time { return time.Now().UTC() }
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v2/products", s.products)
	mux.HandleFunc("POST /v2/assessments/resolve", s.resolve)
	s.registerCLI(mux)
	s.registerDist(mux)
	s.registerReview(mux)
	s.registerEntry(mux)
	s.registerGitHub(mux)
	s.registerPanel(mux)
	s.registerAssistant(mux)
	s.registerAvatar(mux)
	s.registerStaff(mux)
	s.registerPrivacy(mux)
	s.registerCompliance(mux)
	return mw.Wrap(mw.CORS(s.cfg.AllowedOrigins, mux), s.log)
}

// resolveFor selects a product and resolves a task for a role+JD (used by the
// CLI exchange). Returns the resolved task (with ProductSlug set).
func (s *Server) resolveFor(ctx context.Context, role, jd string) (taskgen.ResolvedTask, error) {
	cr := core.Role(role)
	if !core.ValidRole(cr) {
		return taskgen.ResolvedTask{}, fmt.Errorf("unknown role %q", role)
	}
	products, err := s.reg.Products(ctx)
	if err != nil {
		return taskgen.ResolvedTask{}, err
	}
	choice, err := engine.Select(cr, jd, products)
	if err != nil {
		return taskgen.ResolvedTask{}, err
	}
	bank, err := s.bankFor(choice.Product.Slug, filepath.Join(s.productsRoot, choice.Product.Slug))
	if err != nil {
		return taskgen.ResolvedTask{}, err
	}
	gen := taskgen.New(bank, s.cache, s.provider, s.poolSize)
	return gen.Resolve(ctx, choice.Product.Slug, cr, jd)
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
