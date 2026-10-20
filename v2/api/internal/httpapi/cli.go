package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mindfries/v2/api/internal/bundle"
	"github.com/mindfries/v2/api/internal/delivery"
	"github.com/mindfries/v2/api/internal/engine"
	"github.com/mindfries/v2/api/internal/session"
)

// The CLI contract is mounted under this prefix (matches v2/cli fakeapi.Prefix).
const cliPrefix = "/v2/cli"

// minCLIVersion is the lowest CLI version accepted (0.0.0 = accept all for now).
const minCLIVersion = "0.0.0"
const latestCLIVersion = "0.1.0"
const versionHeader = "Mindfries-CLI-Version"

// registerCLI adds the CLI session-contract routes. Only active when the server
// was built with a session store.
func (s *Server) registerCLI(mux *http.ServeMux) {
	if s.sessions == nil {
		return
	}
	mux.HandleFunc("GET "+cliPrefix+"/version", s.cliVersion)
	mux.HandleFunc("POST "+cliPrefix+"/sessions/exchange", s.cliExchange)
	mux.HandleFunc("GET "+cliPrefix+"/session", s.cliSession)
	mux.HandleFunc("GET "+cliPrefix+"/session/bundle", s.cliBundle)
	mux.HandleFunc("GET "+cliPrefix+"/session/bundle/archive", s.cliArchive)
	mux.HandleFunc("POST "+cliPrefix+"/session/heartbeat", s.cliHeartbeat)
	mux.HandleFunc("POST "+cliPrefix+"/session/events", s.cliEvents)
	mux.HandleFunc("POST "+cliPrefix+"/session/snapshots", s.cliSnapshots)
	mux.HandleFunc("POST "+cliPrefix+"/session/revoke", s.cliRevoke)
	// Mint a setup code (stands in for the dashboard/company portal until the
	// V1 seam is wired). Guarded by V2_ADMIN_TOKEN.
	mux.HandleFunc("POST /v2/admin/setup-codes", s.adminMintCode)
}

func (s *Server) adminMintCode(w http.ResponseWriter, r *http.Request) {
	tok := os.Getenv("V2_ADMIN_TOKEN")
	if tok == "" || r.Header.Get("X-Admin-Token") != tok {
		problem(w, http.StatusForbidden, "forbidden", "valid X-Admin-Token required")
		return
	}
	var req struct {
		Code            string `json:"code"`
		AssessmentTitle string `json:"assessment_title"`
		Role            string `json:"role"`
		JD              string `json:"jd"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Role == "" {
		problem(w, http.StatusBadRequest, "malformed", "role is required")
		return
	}
	if req.Code == "" {
		req.Code = randHex(6)
	}
	if req.DurationMinutes == 0 {
		req.DurationMinutes = 90
	}
	c := session.SetupCode{
		Code: req.Code, AssessmentTitle: req.AssessmentTitle, Role: req.Role, JD: req.JD,
		DurationMinutes: req.DurationMinutes, ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	if err := s.sessions.CreateSetupCode(r.Context(), c); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"setup_code": c.Code, "expires_at": c.ExpiresAt})
}

func (s *Server) cliVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"minimum_version": minCLIVersion, "latest_version": latestCLIVersion,
	})
}

func (s *Server) cliExchange(w http.ResponseWriter, r *http.Request) {
	if !s.checkVersion(w, r) {
		return
	}
	var req struct {
		SetupCode string `json:"setup_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid request body")
		return
	}
	now := time.Now().UTC()
	code, err := s.sessions.TakeSetupCode(r.Context(), req.SetupCode, now)
	if errors.Is(err, session.ErrNotFound) {
		problem(w, http.StatusGone, "setup_code_invalid", "setup code not found")
		return
	}
	if errors.Is(err, session.ErrCodeUnusable) {
		problem(w, http.StatusGone, "setup_code_used", "setup code is used or expired")
		return
	}
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	rt, err := s.resolveFor(r, code.Role, code.JD)
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "no_task", err.Error())
		return
	}

	sess := session.Session{
		ID:              "ses_" + randHex(12),
		Token:           randHex(32),
		AssessmentTitle: code.AssessmentTitle,
		Role:            code.Role,
		JDHash:          engine.HashJD(code.JD),
		ProductSlug:     rt.ProductSlug,
		BankTaskID:      rt.BankTaskID,
		TaskVersion:     rt.TaskVersion,
		Brief:           rt.Brief,
		Status:          "active",
		StartedAt:       now,
		EndsAt:          now.Add(time.Duration(code.DurationMinutes) * time.Minute),
	}
	if err := s.sessions.CreateSession(r.Context(), sess); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_token": sess.Token,
		"expires_at":    sess.EndsAt,
		"session":       toWireSession(sess, 0),
	})
}

func (s *Server) cliSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	acked, _ := s.sessions.LastAckedSeq(r.Context(), sess.ID)
	writeJSON(w, http.StatusOK, toWireSession(sess, acked))
}

func (s *Server) cliBundle(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedActive(w, r)
	if !ok {
		return
	}
	m, _, err := s.buildBundle(sess)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) cliArchive(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedActive(w, r)
	if !ok {
		return
	}
	_, archive, err := s.buildBundle(sess)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func (s *Server) cliHeartbeat(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	stop := false
	status := sess.Status
	if status == "active" && now.After(sess.EndsAt) {
		_ = s.sessions.SetStatus(r.Context(), sess.ID, "expired")
		status, stop = "expired", true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"server_time":    now,
		"session_status": status,
		"ends_at":        sess.EndsAt,
		"stop":           stop,
	})
}

func (s *Server) cliEvents(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedActive(w, r)
	if !ok {
		return
	}
	var batch struct {
		Events []session.Event `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid event batch")
		return
	}
	acked, stored, dup, err := s.sessions.AppendEvents(r.Context(), sess.ID, batch.Events)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"acked_through_seq": acked, "stored": stored, "duplicates": dup,
	})
}

func (s *Server) cliSnapshots(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedActive(w, r)
	if !ok {
		return
	}
	var snap session.Snapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid snapshot")
		return
	}
	if err := s.sessions.SaveSnapshot(r.Context(), sess.ID, snap); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cliRevoke(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	_ = s.sessions.SetStatus(r.Context(), sess.ID, "revoked")
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

// buildBundle assembles the session's workspace and signs it (dev key for now).
func (s *Server) buildBundle(sess session.Session) (bundle.Manifest, []byte, error) {
	productDir := filepath.Join(s.productsRoot, sess.ProductSlug)
	ws, err := delivery.Assemble(productDir, sess.BankTaskID, sess.Brief, sess.TaskVersion)
	if err != nil {
		return bundle.Manifest{}, nil, err
	}
	return bundle.Build(ws, bundle.DevKeyID, bundle.DevKey())
}

func (s *Server) authed(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	if !s.checkVersion(w, r) {
		return session.Session{}, false
	}
	tok := bearer(r)
	if tok == "" {
		problem(w, http.StatusUnauthorized, "token_missing", "no session token")
		return session.Session{}, false
	}
	sess, err := s.sessions.SessionByToken(r.Context(), tok)
	if err != nil {
		problem(w, http.StatusUnauthorized, "token_invalid", "unknown session token")
		return session.Session{}, false
	}
	return sess, true
}

func (s *Server) authedActive(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	sess, ok := s.authed(w, r)
	if !ok {
		return session.Session{}, false
	}
	if sess.Status != "active" {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "session_not_active", "detail": "the session has ended", "session_status": sess.Status,
		})
		return session.Session{}, false
	}
	return sess, true
}

func (s *Server) checkVersion(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get(versionHeader) == "" {
		problem(w, http.StatusBadRequest, "malformed", "missing "+versionHeader)
		return false
	}
	// minCLIVersion is 0.0.0 for now, so any present version is accepted.
	return true
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) > len(p) && h[:len(p)] == p {
		return h[len(p):]
	}
	return ""
}

func problem(w http.ResponseWriter, code int, kind, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": kind, "detail": detail})
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- wire session (matches the CLI's api.Session json) ---

func toWireSession(s session.Session, lastAcked int64) map[string]any {
	return map[string]any{
		"id":               s.ID,
		"assessment_id":    "asm_" + s.ID, // one assessment per session for now
		"assessment_title": s.AssessmentTitle,
		"status":           s.Status,
		"started_at":       s.StartedAt,
		"ends_at":          s.EndsAt,
		"policy": map[string]any{
			"assistant_mode":         "off",
			"outside_agents_allowed": false,
		},
		"intervals": map[string]any{
			"heartbeat_seconds": 30, "snapshot_seconds": 120, "agent_scan_seconds": 180,
		},
		"limits": map[string]any{
			"max_events_per_batch": 500, "max_batch_bytes": 1 << 20,
			"max_snapshot_bytes": 2 << 20, "max_bundle_bytes": int64(50) << 20,
		},
		"last_acked_seq": lastAcked,
	}
}
