package fakeapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

// SessionOptions describe the assessment a setup code starts.
type SessionOptions struct {
	Title     string
	Duration  time.Duration // default 90 minutes
	CodeTTL   time.Duration // default 10 minutes
	Policy    api.Policy    // default: code allowed, 200k tokens
	Intervals api.Intervals // default 30s / 120s / 180s
	Limits    api.Limits
	Task      *Task // default DemoTask()
}

type setupCode struct {
	code      string
	expiresAt time.Time
	used      bool
	opts      SessionOptions
}

// session is the server's view of one candidate session.
type session struct {
	api.Session
	token   string
	revoked bool
	task    *Task
	// Filled by later parts of the contract.
	ext sessionExt
}

func randomID(prefix string, n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// codeAlphabet leaves out 0/O and 1/I.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newCode() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	var sb strings.Builder
	for i, c := range b {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(codeAlphabet[int(c)%len(codeAlphabet)])
	}
	return sb.String()
}

// MintSetupCode creates a single-use setup code, as the dashboard does when
// the candidate starts an assessment.
func (s *Server) MintSetupCode(o SessionOptions) string {
	if o.Title == "" {
		o.Title = "Demo assessment"
	}
	if o.Duration == 0 {
		o.Duration = 90 * time.Minute
	}
	if o.CodeTTL == 0 {
		o.CodeTTL = 10 * time.Minute
	}
	if o.Policy.AssistantMode == "" {
		o.Policy.AssistantMode = api.ModeCodeAllowed
	}
	if o.Policy.AssistantBudget == nil {
		o.Policy.AssistantBudget = &api.Budget{TokensTotal: 200_000, TokensRemaining: 200_000}
	}
	if o.Intervals.HeartbeatSeconds == 0 {
		o.Intervals = api.Intervals{HeartbeatSeconds: 30, SnapshotSeconds: 120, AgentScanSeconds: 180}
	}
	if o.Limits.MaxEventsPerBatch == 0 {
		o.Limits = api.Limits{MaxEventsPerBatch: 500, MaxBatchBytes: 1 << 20, MaxSnapshotBytes: 2 << 20, MaxBundleBytes: 50 << 20}
	}
	if o.Task == nil {
		o.Task = DemoTask()
	}
	c := newCode()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[c] = &setupCode{code: c, expiresAt: s.now().Add(o.CodeTTL), opts: o}
	return c
}

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /sessions/exchange", s.handleExchange)
		route("GET /session", s.authed(s.handleGetSession))
		route("POST /session/revoke", s.authed(s.handleRevoke))
	})
	registerAdmin(func(s *Server, mux *http.ServeMux) {
		mux.HandleFunc("POST /__admin/setup-codes", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Title           string `json:"title"`
				DurationMinutes int    `json:"duration_minutes"`
				AssistantMode   string `json:"assistant_mode"`
			}
			if r.ContentLength != 0 && !decode(w, r, &in) {
				return
			}
			o := SessionOptions{Title: in.Title, Duration: time.Duration(in.DurationMinutes) * time.Minute}
			o.Policy.AssistantMode = in.AssistantMode
			writeJSON(w, http.StatusCreated, map[string]string{"setup_code": s.MintSetupCode(o)})
		})
	})
}

func (s *Server) handleExchange(w http.ResponseWriter, r *http.Request) {
	var in api.ExchangeRequest
	if !decode(w, r, &in) {
		return
	}
	if in.Device.OS == "" || in.Device.Arch == "" {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "device is required", "")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.codes[strings.ToUpper(strings.TrimSpace(in.SetupCode))]
	switch {
	case !ok:
		problem(w, http.StatusGone, api.CodeSetupCodeInvalid, "This setup code isn't recognised", "")
		return
	case c.used:
		problem(w, http.StatusGone, api.CodeSetupCodeUsed, "This setup code has already been used", "")
		return
	case s.now().After(c.expiresAt):
		problem(w, http.StatusGone, api.CodeSetupCodeExpired, "This setup code has expired", "")
		return
	}
	c.used = true
	now := s.now()
	sess := &session{
		Session: api.Session{
			ID:              randomID("ses_", 8),
			AssessmentID:    randomID("asm_", 6),
			AssessmentTitle: c.opts.Title,
			Status:          api.StatusActive,
			StartedAt:       now,
			EndsAt:          now.Add(c.opts.Duration),
			Policy:          c.opts.Policy,
			Intervals:       c.opts.Intervals,
			Limits:          c.opts.Limits,
		},
		token: randomID("mft_", 24),
		task:  c.opts.Task,
	}
	budget := *c.opts.Policy.AssistantBudget
	sess.Policy.AssistantBudget = &budget
	s.sessions[sess.ID] = sess
	writeJSON(w, http.StatusOK, api.ExchangeResponse{SessionToken: sess.token, ExpiresAt: sess.EndsAt, Session: sess.view()})
}

// view is the session as the contract returns it.
func (ss *session) view() api.Session {
	v := ss.Session
	if ss.Policy.AssistantBudget != nil {
		b := *ss.Policy.AssistantBudget
		v.Policy.AssistantBudget = &b
	}
	v.LastAckedSeq = ss.ext.lastSeq
	return v
}

// authed resolves the bearer token. The handler runs with s.mu held.
func (s *Server) authed(h func(w http.ResponseWriter, r *http.Request, ss *session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || tok == "" {
			problem(w, http.StatusUnauthorized, api.CodeTokenMissing, "No session token", "")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var ss *session
		for _, cand := range s.sessions {
			if cand.token == tok {
				ss = cand
				break
			}
		}
		switch {
		case ss == nil:
			problem(w, http.StatusUnauthorized, api.CodeTokenInvalid, "Session token not recognised", "")
			return
		case ss.revoked:
			problem(w, http.StatusUnauthorized, api.CodeTokenRevoked, "This session token was revoked", "")
			return
		case s.now().After(ss.EndsAt):
			if ss.Status == api.StatusActive {
				ss.Status = api.StatusExpired
			}
			problem(w, http.StatusUnauthorized, api.CodeTokenExpired, "This session has ended", "")
			return
		}
		h(w, r, ss)
	}
}

// requireActive answers session_not_active unless the session is active.
func requireActive(w http.ResponseWriter, ss *session) bool {
	if ss.Status == api.StatusActive {
		return true
	}
	writeProblem(w, &api.Problem{Code: api.CodeSessionNotActive, Title: "This session is no longer active", Status: http.StatusConflict, SessionStatus: ss.Status})
	return false
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request, ss *session) {
	writeJSON(w, http.StatusOK, ss.view())
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request, ss *session) {
	var in api.RevokeRequest
	if !decode(w, r, &in) {
		return
	}
	switch in.Reason {
	case "stop", "submitted", "expired":
	default:
		problem(w, http.StatusBadRequest, api.CodeMalformed, "reason must be stop, submitted or expired", "")
		return
	}
	ss.revoked = true
	if ss.Status == api.StatusActive {
		if in.Reason == "expired" {
			ss.Status = api.StatusExpired
		} else if in.Reason == "stop" {
			ss.Status = api.StatusRevoked
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// EndSession ends a session from the "portal", as a recruiter or the
// timer would.
func (s *Server) EndSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil {
		ss.Status = api.StatusRevoked
	}
}

// SessionInfo returns the server's view of a session, for tests.
func (s *Server) SessionInfo(id string) (api.Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.sessions[id]
	if ss == nil {
		return api.Session{}, false
	}
	return ss.view(), true
}

// Revoked reports whether a session's token was revoked.
func (s *Server) Revoked(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.sessions[id]
	return ss != nil && ss.revoked
}

// OnlySessionID returns the ID of the only session, for tests.
func (s *Server) OnlySessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.sessions {
		return id
	}
	return ""
}
