package httpapi

import "net/http"

// registerPanelData adds what the panel reads and saves: the live feed, its
// configuration and the artifacts. Filled in by the tracker and panel API.
func (s *Server) registerPanelData(mux *http.ServeMux) {}
