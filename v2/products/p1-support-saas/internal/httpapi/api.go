// Package httpapi exposes the support domain over a JSON HTTP API. It is a thin
// layer: every rule lives in package support. Routes use the Go 1.22 method +
// wildcard mux so no third-party router is needed.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mindfries/products/p1-support-saas/internal/support"
)

// New returns an http.Handler serving the support API over svc.
func New(svc *support.Service) http.Handler {
	a := &api{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tickets", a.createTicket)
	mux.HandleFunc("GET /tickets", a.listTickets)
	mux.HandleFunc("GET /tickets/{id}", a.getTicket)
	mux.HandleFunc("POST /tickets/{id}/reply", a.reply)
	mux.HandleFunc("POST /tickets/{id}/assign", a.assign)
	mux.HandleFunc("POST /tickets/{id}/status", a.setStatus)
	mux.HandleFunc("POST /tickets/{id}/rate", a.rate)
	mux.HandleFunc("GET /tickets/{id}/suggestions", a.suggestions)
	mux.HandleFunc("GET /sla/breaches", a.breaches)
	mux.HandleFunc("GET /metrics", a.metrics)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

type api struct{ svc *support.Service }

func (a *api) createTicket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subject        string           `json:"subject"`
		RequesterEmail string           `json:"requester_email"`
		Priority       support.Priority `json:"priority"`
		Body           string           `json:"body"`
	}
	if !decode(w, r, &req) {
		return
	}
	t, err := a.svc.Create(req.Subject, req.RequesterEmail, req.Priority, req.Body)
	respond(w, t, err, http.StatusCreated)
}

func (a *api) listTickets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := support.TicketFilter{
		Status:     support.Status(q.Get("status")),
		Priority:   support.Priority(q.Get("priority")),
		AssigneeID: q.Get("assignee"),
		TeamID:     q.Get("team"),
		Unassigned: q.Get("unassigned") == "true",
	}
	ts, err := a.svc.Store().ListTickets(f)
	respond(w, ts, err, http.StatusOK)
}

func (a *api) getTicket(w http.ResponseWriter, r *http.Request) {
	t, err := a.svc.Store().GetTicket(r.PathValue("id"))
	respond(w, t, err, http.StatusOK)
}

func (a *api) reply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorType support.AuthorType `json:"author_type"`
		AuthorID   string             `json:"author_id"`
		Body       string             `json:"body"`
	}
	if !decode(w, r, &req) {
		return
	}
	t, err := a.svc.Reply(r.PathValue("id"), req.AuthorType, req.AuthorID, req.Body)
	respond(w, t, err, http.StatusOK)
}

func (a *api) assign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		Auto    bool   `json:"auto"`
	}
	if !decode(w, r, &req) {
		return
	}
	var (
		t   support.Ticket
		err error
	)
	if req.Auto {
		t, err = a.svc.AutoAssign(r.PathValue("id"))
	} else {
		t, err = a.svc.Assign(r.PathValue("id"), req.AgentID)
	}
	respond(w, t, err, http.StatusOK)
}

func (a *api) setStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status support.Status `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	t, err := a.svc.SetStatus(r.PathValue("id"), req.Status)
	respond(w, t, err, http.StatusOK)
}

func (a *api) rate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Score int `json:"score"`
	}
	if !decode(w, r, &req) {
		return
	}
	t, err := a.svc.Rate(r.PathValue("id"), req.Score)
	respond(w, t, err, http.StatusOK)
}

func (a *api) suggestions(w http.ResponseWriter, r *http.Request) {
	sug, err := a.svc.Suggest(r.PathValue("id"), 3)
	respond(w, sug, err, http.StatusOK)
}

func (a *api) breaches(w http.ResponseWriter, r *http.Request) {
	ts, err := a.svc.BreachedTickets()
	respond(w, ts, err, http.StatusOK)
}

func (a *api) metrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	m, err := a.svc.Metrics(support.TicketFilter{
		Status:   support.Status(q.Get("status")),
		Priority: support.Priority(q.Get("priority")),
		TeamID:   q.Get("team"),
	})
	respond(w, m, err, http.StatusOK)
}

// --- helpers ---

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

// respond maps domain errors to status codes: not-found -> 404, any other
// error -> 400 (they are validation errors from the service), success -> ok.
func respond(w http.ResponseWriter, v any, err error, ok int) {
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, support.ErrNotFound) {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, ok, v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
