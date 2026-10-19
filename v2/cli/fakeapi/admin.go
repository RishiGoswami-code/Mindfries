package fakeapi

import (
	"net/http"
)

// adminRoutes are the fake's own controls, for cmd/fakeapi and manual
// testing. They are not part of the contract.
func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /__admin/offline", func(w http.ResponseWriter, r *http.Request) {
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
		a(s, mux)
	}
}

var adminRegistered []func(*Server, *http.ServeMux)

func registerAdmin(f func(*Server, *http.ServeMux)) { adminRegistered = append(adminRegistered, f) }
