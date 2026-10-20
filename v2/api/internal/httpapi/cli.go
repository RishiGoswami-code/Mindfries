package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	mux.HandleFunc("POST "+cliPrefix+"/session/begin", s.cliBegin)
	mux.HandleFunc("GET "+cliPrefix+"/session/monitoring", s.cliMonitoring)
	mux.HandleFunc("GET "+cliPrefix+"/session/bundle", s.cliBundle)
	mux.HandleFunc("GET "+cliPrefix+"/session/bundle/archive", s.cliArchive)
	mux.HandleFunc("POST "+cliPrefix+"/session/heartbeat", s.cliHeartbeat)
	mux.HandleFunc("POST "+cliPrefix+"/session/events", s.cliEvents)
	mux.HandleFunc("POST "+cliPrefix+"/session/snapshots", s.cliSnapshots)
	mux.HandleFunc("POST "+cliPrefix+"/session/submission", s.cliSubmit)
	mux.HandleFunc("GET "+cliPrefix+"/session/submission", s.cliSubmissionStatus)
	mux.HandleFunc("PUT "+cliPrefix+"/session/submission/{id}/bundle", s.cliSubmitBundle)
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
		AssessmentID    string `json:"assessment_id"`
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
		req.Code = newSetupCode()
	} else {
		req.Code = normalizeSetupCode(req.Code)
	}
	if req.DurationMinutes == 0 {
		req.DurationMinutes = 90
	}

	// Resolve-at-invite: prove we can serve this role/JD before minting a code.
	// If no product covers it, the admin learns here, at assign time — a code is
	// never issued for an assessment the candidate couldn't start. (This also
	// warms the task cache so the candidate's setup is a cache hit.)
	rt, err := s.resolveFor(r, req.Role, req.JD)
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "no_coverage", err.Error())
		return
	}

	c := session.SetupCode{
		Code: req.Code, AssessmentID: req.AssessmentID, AssessmentTitle: req.AssessmentTitle,
		Role: req.Role, JD: req.JD,
		DurationMinutes: req.DurationMinutes, ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	if err := s.sessions.CreateSetupCode(r.Context(), c); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"setup_code": c.Code, "expires_at": c.ExpiresAt,
		"product_slug": rt.ProductSlug, "task_title": rt.Title, "task_kind": rt.Kind,
	})
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
	code, err := s.sessions.TakeSetupCode(r.Context(), normalizeSetupCode(req.SetupCode), now)
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
		AssessmentID:    code.AssessmentID,
		AssessmentTitle: code.AssessmentTitle,
		Role:            code.Role,
		JDHash:          engine.HashJD(code.JD),
		ProductSlug:     rt.ProductSlug,
		BankTaskID:      rt.BankTaskID,
		TaskVersion:     rt.TaskVersion,
		Brief:           rt.Brief,
		DurationMinutes: code.DurationMinutes,
		Status:          "ready", // the clock starts at begin, not at setup
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

// cliBegin starts the clock: ready -> active, setting the time budget.
func (s *Server) cliBegin(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
	if !ok {
		return
	}
	// Idempotent once begun.
	if sess.Status == "active" {
		writeJSON(w, http.StatusOK, toWireSession(sess, 0))
		return
	}
	var req struct {
		IDE    string `json:"ide"`
		Checks []any  `json:"checks"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // ide/checks are evidence, not gates here
	now := time.Now().UTC()
	ends := now.Add(time.Duration(sess.DurationMinutes) * time.Minute)
	if err := s.sessions.Begin(r.Context(), sess.ID, now, ends); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	sess.Status, sess.StartedAt, sess.EndsAt = "active", now, ends
	writeJSON(w, http.StatusOK, toWireSession(sess, 0))
}

// cliMonitoring reports live monitoring state. Monitoring is off by default in
// this build, so the CLI's begin never blocks on it.
func (s *Server) cliMonitoring(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authed(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"panel_open": false, "camera": false, "screen": false, "audio": false,
	})
}

func (s *Server) cliBundle(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
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
	sess, ok := s.authedLive(w, r)
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
	sess, ok := s.authedLive(w, r)
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
	sess, ok := s.authedLive(w, r)
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

func (s *Server) cliSubmit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
	if !ok {
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	var man struct {
		TaskVersion     string `json:"task_version"`
		Branch          string `json:"branch"`
		BaseCommit      string `json:"base_commit"`
		HeadCommit      string `json:"head_commit"`
		CommitMessage   string `json:"commit_message"`
		BundleSHA256    string `json:"bundle_sha256"`
		BundleBytes     int64  `json:"bundle_bytes"`
		TaskFilesIntact bool   `json:"task_files_intact"`
	}
	if err := json.Unmarshal(raw, &man); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid submission manifest")
		return
	}
	if man.HeadCommit == "" || man.BaseCommit == "" || man.BundleSHA256 == "" || man.TaskVersion == "" {
		problem(w, http.StatusBadRequest, "malformed", "manifest needs task_version, base_commit, head_commit, bundle_sha256")
		return
	}
	// Idempotent: a session has one submission.
	if existing, err := s.sessions.SubmissionBySession(r.Context(), sess.ID); err == nil {
		writeJSON(w, http.StatusCreated, toWireSubmission(existing))
		return
	}
	sub := session.Submission{
		ID: "sub_" + randHex(10), SessionID: sess.ID, TaskVersion: man.TaskVersion, Branch: man.Branch,
		BaseCommit: man.BaseCommit, HeadCommit: man.HeadCommit, CommitMessage: man.CommitMessage,
		BundleSHA256: man.BundleSHA256, BundleBytes: man.BundleBytes, TaskFilesIntact: man.TaskFilesIntact,
		Manifest: raw, Status: "awaiting_bundle", CreatedAt: time.Now().UTC(),
	}
	if err := s.sessions.CreateSubmission(r.Context(), sub); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toWireSubmission(sub))
}

func (s *Server) cliSubmissionStatus(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	sub, err := s.sessions.SubmissionBySession(r.Context(), sess.ID)
	if err != nil {
		problem(w, http.StatusNotFound, "not_found", "no submission for this session")
		return
	}
	writeJSON(w, http.StatusOK, toWireSubmission(sub))
}

func (s *Server) cliSubmitBundle(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	sub, err := s.sessions.SubmissionByID(r.Context(), r.PathValue("id"))
	if err != nil || sub.SessionID != sess.ID {
		problem(w, http.StatusNotFound, "not_found", "submission not found")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 50<<20))
	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if got != sub.BundleSHA256 {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "bundle_mismatch", "detail": "uploaded bundle does not match the manifest",
			"mismatches": []map[string]string{{"what": "bundle_sha256", "expected": sub.BundleSHA256, "actual": got}},
		})
		return
	}
	// Integrity verified (bundle bytes match the manifest). Deep verification —
	// unpacking the git bundle and running the task's harness on head_commit —
	// is the downstream grading step; the bundle is stored for it.
	now := time.Now().UTC()
	if err := s.sessions.CompleteSubmission(r.Context(), sub.ID, body, "verified", now); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	_ = s.sessions.SetStatus(r.Context(), sess.ID, "submitted")
	// Reflect the hand-in on the V1 assessment so the candidate dashboard and
	// admin stop offering another setup (one submission). Best-effort.
	if err := s.sessions.MarkAssessmentSubmitted(r.Context(), sess.AssessmentID); err != nil {
		log.Printf("submit: couldn't mark assessment %s submitted: %v", sess.AssessmentID, err)
	}
	sub.Status, sub.VerifiedAt = "verified", &now
	writeJSON(w, http.StatusOK, toWireSubmission(sub))
}

func toWireSubmission(sub session.Submission) map[string]any {
	m := map[string]any{"id": sub.ID, "status": sub.Status, "created_at": sub.CreatedAt, "head_commit": sub.HeadCommit}
	if sub.VerifiedAt != nil {
		m["verified_at"] = *sub.VerifiedAt
	}
	return m
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

// buildBundle assembles the session's workspace and signs it. It uses the
// configured production key when set, otherwise the dev key (dev/test).
func (s *Server) buildBundle(sess session.Session) (bundle.Manifest, []byte, error) {
	productDir := filepath.Join(s.productsRoot, sess.ProductSlug)
	ws, err := delivery.Assemble(productDir, sess.BankTaskID, sess.Brief, sess.TaskVersion)
	if err != nil {
		return bundle.Manifest{}, nil, err
	}
	keyID, key := s.signKeyID, s.signKey
	if key == nil {
		keyID, key = bundle.DevKeyID, bundle.DevKey()
	}
	return bundle.Build(ws, keyID, key)
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

// authedLive allows a session that is set up (ready) or running (active) —
// bundle download and evidence happen during setup, before the clock starts.
func (s *Server) authedLive(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	sess, ok := s.authed(w, r)
	if !ok {
		return session.Session{}, false
	}
	if sess.Status != "active" && sess.Status != "ready" {
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

// setupCodeAlphabet leaves out 0/O and 1/I. Matches the CLI (and fakeapi).
const setupCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// newSetupCode generates a code in the CLI's expected format: three groups of
// four, e.g. K7QM-2XRP-9WDA. The CLI validates this shape, so it must match.
func newSetupCode() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	var sb strings.Builder
	for i, c := range b {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(setupCodeAlphabet[int(c)%len(setupCodeAlphabet)])
	}
	return sb.String()
}

// normalizeSetupCode canonicalizes a code the same way the CLI does before it
// sends it, so stored and looked-up codes match regardless of how it was typed.
func normalizeSetupCode(c string) string {
	c = strings.ToUpper(strings.Join(strings.Fields(c), ""))
	if len(c) == 12 && !strings.Contains(c, "-") {
		c = c[0:4] + "-" + c[4:8] + "-" + c[8:12]
	}
	return c
}

// --- wire session (matches the CLI's api.Session json) ---

func toWireSession(s session.Session, lastAcked int64) map[string]any {
	return map[string]any{
		"id":               s.ID,
		"assessment_id":    "asm_" + s.ID, // one assessment per session for now
		"assessment_title": s.AssessmentTitle,
		"status":           s.Status,
		"duration_minutes": s.DurationMinutes,
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
