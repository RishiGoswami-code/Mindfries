package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/bundle"
	"github.com/mindfries/v2/api/internal/delivery"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

// The CLI contract is mounted under this prefix (matches v2/cli fakeapi.Prefix).
const cliPrefix = "/v2/cli"

// registerCLI adds the CLI session-contract routes. Only active when the server
// was built with a session store.
func (s *Server) registerCLI(mux *http.ServeMux) {
	if s.sessions == nil {
		return
	}
	mux.HandleFunc("GET "+cliPrefix+"/version", s.cliVersion)
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
	// What the events mean for the panel's tracker. An event sent twice makes
	// one line (the item's dedupe key is the event's seq).
	for _, e := range batch.Events {
		if e.Type == "check.run" {
			s.keepCheckRun(r.Context(), sess.ID, e)
		}
		for _, it := range activity.FromEvent(activity.Event{Seq: e.Seq, At: e.OccurredAt, Type: e.Type, Data: e.Data}) {
			s.track(r.Context(), sess.ID, it)
		}
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
		problemWith(w, http.StatusUnprocessableEntity, "bundle_mismatch", "uploaded bundle does not match the manifest",
			map[string]any{"mismatches": []map[string]string{{"what": "bundle_sha256", "expected": sub.BundleSHA256, "actual": got}}})
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
		s.log.Error("submit: couldn't mark the assessment submitted", "assessment", sess.AssessmentID, "err", err)
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

// cliRevoke ends the CLI's use of its token. The reason says why: `stop` (the
// candidate ended it) revokes the session; `submitted` and `expired` leave the
// session as it is (it's already submitted or out of time). Either way the
// token stops working. A session that has finished is never changed.
func (s *Server) cliRevoke(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req)
	ctx := r.Context()
	if sess.Live() {
		status := "revoked"
		if req.Reason == "expired" {
			status = "expired"
		}
		if err := s.sessions.SetStatus(ctx, sess.ID, status); err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
	// Whatever the reason, the token is dead: nothing else can use it.
	if err := s.sessions.RotateToken(ctx, sess.ID, randHex(32)); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
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

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// keepCheckRun stores a check.run event for the panel's Testing tab.
func (s *Server) keepCheckRun(ctx context.Context, sessionID string, e session.Event) {
	if s.panelStore == nil {
		return
	}
	var d struct {
		Command    string   `json:"command"`
		Passed     bool     `json:"passed"`
		Failing    []string `json:"failing"`
		Passing    []string `json:"passing"`
		OutputTail string   `json:"output_tail"`
	}
	if json.Unmarshal(e.Data, &d) != nil || d.Command == "" {
		return
	}
	if len(d.OutputTail) > 4096 {
		d.OutputTail = d.OutputTail[len(d.OutputTail)-4096:]
	}
	seq := e.Seq
	if _, err := s.panelStore.SaveCheckRun(ctx, store.CheckRun{SessionID: sessionID, EventSeq: &seq, RanAt: e.OccurredAt,
		Command: d.Command, Passed: d.Passed, Failing: d.Failing, Passing: d.Passing, OutputTail: d.OutputTail}); err != nil {
		s.log.Error("check run: store failed", "session", sessionID, "err", err)
	}
}
