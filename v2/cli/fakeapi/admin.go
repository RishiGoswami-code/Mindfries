package fakeapi

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
)

// adminRoutes are the fake's own controls, for cmd/fakeapi and manual
// testing. They are not part of the contract. When Config.AdminToken is
// set (as it should be on any deployment others can reach), every admin
// request needs it as a bearer token.
func (s *Server) adminRoutes(mux *http.ServeMux) {
	admin := http.NewServeMux()
	admin.HandleFunc("POST /__admin/offline", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			On bool `json:"on"`
		}
		if !decode(w, r, &in) {
			return
		}
		s.SetOffline(in.On)
		writeJSON(w, http.StatusOK, in)
	})
	for _, a := range adminRegistered {
		a(s, admin)
	}
	mux.Handle("/__admin/", s.requireAdmin(admin))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	})
}

func (s *Server) requireAdmin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := s.cfg.AdminToken; want != "" {
			got, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
				problem(w, http.StatusUnauthorized, api.CodeTokenInvalid, "Admin token required", "")
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

var adminRegistered []func(*Server, *http.ServeMux)

func registerAdmin(f func(*Server, *http.ServeMux)) { adminRegistered = append(adminRegistered, f) }
