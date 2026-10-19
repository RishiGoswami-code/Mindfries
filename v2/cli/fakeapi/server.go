// Package fakeapi is a scriptable, in-memory server for
// v2/contracts/cli-api.yaml. The CLI is built and tested against it before
// the real V2 API exists; the same end-to-end tests later run against both.
//
// Tests drive it through Go (New, MintSetupCode, the accessors); the
// standalone binary in cmd/fakeapi exposes the same controls under /__admin.
package fakeapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/semver"
)

// Prefix is where the contract's paths are mounted.
const Prefix = "/v2/cli"

// Config sets what the fake serves. Zero values get sensible defaults.
type Config struct {
	MinVersion    string // default 0.0.0
	LatestVersion string // default 0.1.0
	Now           func() time.Time
}

// Server is the fake V2 API.
type Server struct {
	mu  sync.Mutex
	cfg Config

	// Offline makes every contract request fail as if the network were
	// down (the connection is closed without a response).
	offline bool
	// requests counts contract requests by "METHOD /path".
	requests map[string]int
}

// New returns a fake with cfg.
func New(cfg Config) *Server {
	if cfg.MinVersion == "" {
		cfg.MinVersion = "0.0.0"
	}
	if cfg.LatestVersion == "" {
		cfg.LatestVersion = "0.1.0"
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Server{cfg: cfg, requests: map[string]int{}}
}

func (s *Server) now() time.Time { return s.cfg.Now().UTC() }

// SetOffline turns the simulated network outage on or off.
func (s *Server) SetOffline(on bool) {
	s.mu.Lock()
	s.offline = on
	s.mu.Unlock()
}

// Requests returns how many times "METHOD /path" was called.
func (s *Server) Requests(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[key]
}

// Handler serves the contract under Prefix, and the admin controls under
// /__admin.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h http.HandlerFunc) {
		method, path, _ := strings.Cut(pattern, " ")
		mux.HandleFunc(method+" "+Prefix+path, s.wrap(h))
	}
	route("GET /version", s.handleVersion)
	s.routes(route)
	s.adminRoutes(mux)
	return mux
}

// routes is extended by the files that implement each part of the contract.
func (s *Server) routes(route func(string, http.HandlerFunc)) {
	for _, r := range registered {
		r(s, route)
	}
}

var registered []func(*Server, func(string, http.HandlerFunc))

func register(f func(*Server, func(string, http.HandlerFunc))) { registered = append(registered, f) }

// wrap applies what every contract request shares: the outage switch, the
// request count, and the minimum CLI version.
func (s *Server) wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		offline := s.offline
		s.requests[r.Method+" "+strings.TrimPrefix(r.URL.Path, Prefix)]++
		s.mu.Unlock()
		if offline {
			hangUp(w)
			return
		}
		v := r.Header.Get(api.VersionHeader)
		cv, ok := semver.Find(v)
		if v == "" || !ok {
			problem(w, http.StatusBadRequest, api.CodeMalformed, "Missing or bad "+api.VersionHeader, "")
			return
		}
		if semver.Less(cv, semver.MustParse(s.cfg.MinVersion)) {
			p := &api.Problem{Code: api.CodeCLITooOld, Title: "This version of mindfries is too old", Status: http.StatusUpgradeRequired, MinimumVersion: s.cfg.MinVersion}
			writeProblem(w, p)
			return
		}
		h(w, r)
	}
}

// hangUp closes the connection without a response, the way a dropped
// network looks to the client.
func hangUp(w http.ResponseWriter) {
	if hj, ok := w.(http.Hijacker); ok {
		if conn, _, err := hj.Hijack(); err == nil {
			conn.Close()
			return
		}
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.VersionInfo{MinimumVersion: s.cfg.MinVersion, LatestVersion: s.cfg.LatestVersion})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func problem(w http.ResponseWriter, status int, code, title, detail string) {
	writeProblem(w, &api.Problem{Code: code, Title: title, Status: status, Detail: detail})
}

func writeProblem(w http.ResponseWriter, p *api.Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// decode reads a JSON body, answering malformed on failure.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	if err := dec.Decode(v); err != nil {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "Request doesn't match the contract", err.Error())
		return false
	}
	return true
}
