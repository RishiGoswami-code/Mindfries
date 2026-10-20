package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mindfries/v2/api/internal/session"
)

// What the panel reads: the live feed of everything the tracker records.

const (
	feedBacklog   = 500
	feedKeepAlive = 15 * time.Second
)

func (s *Server) registerPanelData(mux *http.ServeMux) {
	mux.HandleFunc("GET /v2/panel/feed", s.panelAuth(s.panelFeed))
	mux.HandleFunc("GET /v2/panel/activity", s.panelAuth(s.panelActivity))
	s.registerPanelConfig(mux)
}

// lastID reads where a feed should resume: Last-Event-ID (what a browser's
// EventSource sends when it reconnects) or ?after=.
func lastID(r *http.Request) int64 {
	v := r.Header.Get("Last-Event-ID")
	if v == "" {
		v = r.URL.Query().Get("after")
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	if n < 0 {
		n = 0
	}
	return n
}

// panelActivity is the tracker as plain JSON, oldest first, for a page that
// isn't streaming.
func (s *Server) panelActivity(w http.ResponseWriter, r *http.Request, sess session.Session) {
	if s.acts == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "the tracker isn't set up on this server")
		return
	}
	limit := 200
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 1000 {
		limit = n
	}
	items, err := s.acts.ActivityAfter(r.Context(), sess.ID, lastID(r), limit)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	counts, _ := s.acts.ActivityCounts(r.Context(), sess.ID)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "counts": counts})
}

// panelFeed streams the tracker as Server-Sent Events: the backlog after
// Last-Event-ID, then each new item as it happens. A dropped connection
// reconnects with the last id it saw and misses nothing.
func (s *Server) panelFeed(w http.ResponseWriter, r *http.Request, sess session.Session) {
	if s.acts == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "the tracker isn't set up on this server")
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		problem(w, http.StatusInternalServerError, "internal", "streaming isn't supported here")
		return
	}
	ctx := r.Context()
	// Subscribe before reading the backlog, so nothing falls between the two.
	live, cancel := s.hub.Subscribe(sess.ID)
	defer cancel()
	last := lastID(r)
	backlog, err := s.acts.ActivityAfter(ctx, sess.ID, last, feedBacklog)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no") // proxies must not hold the stream
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n\n")

	send := func(id int64, raw []byte) bool {
		if id <= last {
			return true // already sent (it was in the backlog and the hub)
		}
		last = id
		if _, err := fmt.Fprintf(w, "id: %d\nevent: item\ndata: %s\n\n", id, raw); err != nil {
			return false
		}
		fl.Flush()
		return true
	}
	for _, it := range backlog {
		raw, _ := json.Marshal(it)
		if !send(it.ID, raw) {
			return
		}
	}
	fmt.Fprintf(w, "event: ready\ndata: {\"session_status\":%q}\n\n", sess.Status)
	fl.Flush()

	tick := time.NewTicker(feedKeepAlive)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case it, open := <-live:
			if !open {
				// Too slow to keep up: close so the browser reconnects and
				// catches up from the database.
				return
			}
			raw, _ := json.Marshal(it)
			if !send(it.ID, raw) {
				return
			}
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
