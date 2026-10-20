package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/ghub"
	"github.com/mindfries/v2/api/internal/policy"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

// B11: what the company portal and Mindfries admin call, server to server.
// They configure an assessment, see its sessions, read the evidence a session
// produced, and end or extend one. (Scoring, the report and the hiring
// decision are built elsewhere; this serves the evidence they read.)
//
// These routes take either the portals' secret or the admin token. A company
// must only ask about its own assessments: the company portal checks that
// before it calls, and passes company_id so the API can check it too.

// StaffStore is what the staff routes read and write beyond the other stores.
type StaffStore interface {
	SessionsForAssessment(ctx context.Context, assessmentID string) ([]session.Session, error)
	ExtendSession(ctx context.Context, id string, endsAt time.Time) error
	ReviewFlag(ctx context.Context, sessionID string, r store.FlagReview) error
	FlagReviews(ctx context.Context, sessionID string) ([]store.FlagReview, error)
	SessionTranscript(ctx context.Context, sessionID string) ([]store.TranscriptRow, error)
	EvidenceByID(ctx context.Context, sessionID, id string) (store.Evidence, bool, error)
}

func (s *Server) registerStaff(mux *http.ServeMux) {
	if s.sessions == nil {
		return
	}
	mux.HandleFunc("GET /v2/portal/assessments/{id}/config", s.staffOnly(s.getAssessmentConfig))
	mux.HandleFunc("PUT /v2/portal/assessments/{id}/config", s.staffOnly(s.putAssessmentConfig))
	mux.HandleFunc("GET /v2/portal/assessments/{id}/sessions", s.staffOnly(s.assessmentSessions))
	mux.HandleFunc("GET /v2/portal/sessions/{id}/evidence", s.staffOnly(s.sessionEvidence))
	mux.HandleFunc("GET /v2/portal/sessions/{id}/media/{evidence_id}", s.staffOnly(s.mediaURL))
	mux.HandleFunc("GET /v2/portal/sessions/{id}/report", s.staffOnly(s.sessionReport))
	mux.HandleFunc("POST /v2/portal/sessions/{id}/end", s.staffOnly(s.endSession))
	mux.HandleFunc("POST /v2/portal/sessions/{id}/extend", s.staffOnly(s.extendSession))
	mux.HandleFunc("POST /v2/portal/sessions/{id}/activity/{activity_id}/review", s.staffOnly(s.reviewActivity))
	mux.HandleFunc("GET /v2/admin/products/{slug}/tasks", s.adminOnly(s.productTasks))
}

// staffOnly lets through the portals' servers and Mindfries admin.
func (s *Server) staffOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.PortalSecret == "" && s.cfg.AdminToken == "" {
			problem(w, http.StatusServiceUnavailable, "not_configured", "no portal secret or admin token is set")
			return
		}
		okPortal := s.cfg.PortalSecret != "" && auth.Same(r.Header.Get("X-Portal-Secret"), s.cfg.PortalSecret)
		okAdmin := s.cfg.AdminToken != "" && auth.Same(r.Header.Get("X-Admin-Token"), s.cfg.AdminToken)
		if !okPortal && !okAdmin {
			problem(w, http.StatusForbidden, "forbidden", "a portal secret or admin token is required")
			return
		}
		h(w, r)
	}
}

// ownSession loads a session and, when the caller names a company, checks it
// is that company's.
func (s *Server) ownSession(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	sess, err := s.sessions.SessionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		problem(w, http.StatusNotFound, "not_found", "no such session")
		return session.Session{}, false
	}
	if c := r.URL.Query().Get("company_id"); c != "" && c != sess.Links.CompanyID {
		problem(w, http.StatusNotFound, "not_found", "no such session") // not yours, and we don't say it exists
		return session.Session{}, false
	}
	return sess, true
}

// ---- assessment configuration -------------------------------------------------

func (s *Server) getAssessmentConfig(w http.ResponseWriter, r *http.Request) {
	if s.configs == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "configuration isn't set up")
		return
	}
	c, found, err := s.configs.AssessmentConfig(r.Context(), r.PathValue("id"))
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": c, "configured": found})
}

func (s *Server) putAssessmentConfig(w http.ResponseWriter, r *http.Request) {
	if s.configs == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "configuration isn't set up")
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		problem(w, http.StatusBadRequest, "malformed", "the assessment id must be a UUID")
		return
	}
	// Fields left out keep their current value (or the default), so a portal
	// can change just the reviewers.
	cur, _, err := s.configs.AssessmentConfig(r.Context(), id)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var in struct {
		UpdatedBy string `json:"updated_by"`
		policy.Config
	}
	in.Config = cur
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&in); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid configuration")
		return
	}
	in.Config.AssessmentID = id
	if why := in.Config.Problem(); why != "" {
		problem(w, http.StatusBadRequest, "malformed", why)
		return
	}
	in.Config.Reviewers, _ = policy.NormaliseReviewers(in.Config.Reviewers)
	if in.UpdatedBy == "" {
		in.UpdatedBy = "portal"
	}
	if err := s.configs.SaveAssessmentConfig(r.Context(), in.Config, in.UpdatedBy); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": in.Config})
}

// ---- sessions -------------------------------------------------------------------

func (s *Server) assessmentSessions(w http.ResponseWriter, r *http.Request) {
	if s.staff == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "not set up")
		return
	}
	ctx := r.Context()
	list, err := s.staff.SessionsForAssessment(ctx, r.PathValue("id"))
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	company := r.URL.Query().Get("company_id")
	out := []map[string]any{}
	for _, sess := range list {
		if company != "" && company != sess.Links.CompanyID {
			continue
		}
		item := map[string]any{
			"session_id": sess.ID, "candidate_id": sess.Links.CandidateID, "status": sess.Status, "role": sess.Role,
			"product": sess.ProductSlug, "task": sess.BankTaskID, "started_at": sess.StartedAt, "ends_at": sess.EndsAt,
			"begun": sess.BegunAt != nil,
		}
		if s.acts != nil {
			if counts, err := s.acts.ActivityCounts(ctx, sess.ID); err == nil {
				item["flags"], item["notices"] = counts["flag"], counts["notice"]
			}
		}
		if s.github != nil {
			if repo, ok, _ := s.github.RepoFor(ctx, sess.ID); ok {
				item["repo"] = map[string]any{"full_name": repo.Owner + "/" + repo.Name, "html_url": repo.HTMLURL}
			}
		}
		if sub, err := s.sessions.SubmissionBySession(ctx, sess.ID); err == nil {
			item["submission"] = map[string]any{"id": sub.ID, "status": sub.Status, "created_at": sub.CreatedAt}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

// endSession ends a session from the portal (the company stopped it).
func (s *Server) endSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.ownSession(w, r)
	if !ok {
		return
	}
	if !sess.Live() {
		problemWith(w, http.StatusConflict, "session_not_active", "the session is already over", map[string]any{"session_status": sess.Status})
		return
	}
	if err := s.sessions.SetStatus(r.Context(), sess.ID, "revoked"); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// extendSession gives a running session more time, up to an hour at once.
func (s *Server) extendSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.ownSession(w, r)
	if !ok {
		return
	}
	var in struct {
		Minutes int `json:"minutes"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in); err != nil || in.Minutes < 1 || in.Minutes > 60 {
		problem(w, http.StatusBadRequest, "malformed", "minutes must be between 1 and 60")
		return
	}
	if !sess.Live() {
		problemWith(w, http.StatusConflict, "session_not_active", "the session is already over", map[string]any{"session_status": sess.Status})
		return
	}
	ends := sess.EndsAt.Add(time.Duration(in.Minutes) * time.Minute)
	if err := s.staff.ExtendSession(r.Context(), sess.ID, ends); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ends_at": ends})
}

// ---- evidence -------------------------------------------------------------------

// sessionEvidence is everything a session recorded, as a reviewer reads it.
// Camera, screen and microphone files are listed (kind, time, size) and
// fetched one at a time through mediaURL, which returns a short-lived address.
func (s *Server) sessionEvidence(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.ownSession(w, r)
	if !ok {
		return
	}
	if !s.audit(w, r, sess.ID, "evidence", "") {
		return
	}
	ctx := r.Context()
	out := map[string]any{"session": map[string]any{
		"id": sess.ID, "assessment_id": sess.AssessmentID, "candidate_id": sess.Links.CandidateID, "company_id": sess.Links.CompanyID,
		"role": sess.Role, "product": sess.ProductSlug, "task": sess.BankTaskID, "task_version": sess.TaskVersion,
		"status": sess.Status, "started_at": sess.StartedAt, "ends_at": sess.EndsAt, "duration_minutes": sess.DurationMinutes,
	}, "config": s.configFor(ctx, sess)}
	fail := func(what string, err error) bool {
		if err != nil {
			problem(w, http.StatusInternalServerError, "internal", what+": "+err.Error())
			return true
		}
		return false
	}
	if s.monitor != nil {
		st, _, err := s.monitor.MonitoringState(ctx, sess.ID)
		if fail("monitoring", err) {
			return
		}
		gaps, err := s.monitor.Gaps(ctx, sess.ID)
		if fail("gaps", err) {
			return
		}
		media, err := s.monitor.EvidenceFor(ctx, sess.ID, "", 5000)
		if fail("media", err) {
			return
		}
		out["monitoring"] = map[string]any{"last_report": st, "gaps": gaps}
		out["media"] = media
	}
	if s.acts != nil {
		items, err := s.acts.ActivityAfter(ctx, sess.ID, 0, 5000)
		if fail("activity", err) {
			return
		}
		out["activity"] = items
		if s.staff != nil {
			reviews, err := s.staff.FlagReviews(ctx, sess.ID)
			if fail("flag reviews", err) {
				return
			}
			out["flag_reviews"] = reviews
			if s.compliance != nil {
				if resp, err := s.compliance.FlagResponses(ctx, sess.ID); err == nil {
					out["flag_responses"] = resp
				}
				if c, ok, _ := s.compliance.ConsentFor(ctx, sess.Links.CandidateID, sess.AssessmentID); ok {
					out["consent"] = c
				}
			}
		}
	}
	if s.panelStore != nil {
		arts, err := s.panelStore.Artifacts(ctx, sess.ID)
		if fail("artifacts", err) {
			return
		}
		out["artifacts"] = arts
		if run, found, _ := s.panelStore.LatestCheckRun(ctx, sess.ID); found {
			out["latest_check_run"] = run
		}
	}
	if s.staff != nil {
		tr, err := s.staff.SessionTranscript(ctx, sess.ID)
		if fail("transcript", err) {
			return
		}
		out["assistant_transcript"] = tr
	}
	if s.github != nil {
		if repo, found, _ := s.github.RepoFor(ctx, sess.ID); found {
			out["repo"] = repoJSON(repo)
		}
	}
	if sub, err := s.sessions.SubmissionBySession(ctx, sess.ID); err == nil {
		out["submission"] = toWireSubmission(sub)
		if g, err := s.sessions.GradeBySubmission(ctx, sub.ID); err == nil {
			out["grade"] = g
		}
	}
	if events, err := s.sessions.Events(ctx, sess.ID); err == nil {
		out["event_count"] = len(events)
	}
	writeJSON(w, http.StatusOK, out)
}

func repoJSON(r ghub.Repo) map[string]any {
	return map[string]any{"full_name": r.Owner + "/" + r.Name, "html_url": r.HTMLURL, "private": r.Private, "reviewers_invited": r.Invited}
}

// mediaURL returns a short-lived address for one camera still, screenshot or
// microphone clip.
func (s *Server) mediaURL(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.ownSession(w, r)
	if !ok {
		return
	}
	if s.staff == nil || s.blobs == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "evidence storage isn't set up")
		return
	}
	if !s.audit(w, r, sess.ID, "media", r.PathValue("evidence_id")) {
		return
	}
	ev, found, err := s.staff.EvidenceByID(r.Context(), sess.ID, r.PathValue("evidence_id"))
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !found {
		problem(w, http.StatusNotFound, "not_found", "no such file (it may have passed its retention period)")
		return
	}
	const ttl = 10 * time.Minute
	u, err := s.blobs.SignedURL(r.Context(), ev.Path, ttl)
	if err != nil {
		problem(w, http.StatusBadGateway, "internal", "couldn't make an address for that file")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": u, "expires_at": s.now().Add(ttl), "content_type": ev.ContentType, "kind": ev.Kind})
}

// reviewActivity records what a reviewer did about a flag or notice.
func (s *Server) reviewActivity(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.ownSession(w, r)
	if !ok {
		return
	}
	var in struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
		Reviewer string `json:"reviewer"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid review")
		return
	}
	switch in.Decision {
	case "noted", "explained", "dismissed", "escalated":
	default:
		problem(w, http.StatusBadRequest, "malformed", "decision must be noted, explained, dismissed or escalated")
		return
	}
	if in.Reviewer == "" || len(in.Reviewer) > 200 || len(in.Note) > 4000 {
		problem(w, http.StatusBadRequest, "malformed", "a review names who made it and keeps its note short")
		return
	}
	aid, err := strconv.ParseInt(r.PathValue("activity_id"), 10, 64)
	if err != nil {
		problem(w, http.StatusBadRequest, "malformed", "bad activity id")
		return
	}
	err = s.staff.ReviewFlag(r.Context(), sess.ID, store.FlagReview{ActivityID: aid, Decision: in.Decision, Note: in.Note, Reviewer: in.Reviewer, ReviewedAt: s.now()})
	if errors.Is(err, session.ErrNotFound) {
		problem(w, http.StatusNotFound, "not_found", "no such item in this session")
		return
	}
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// productTasks lists a product's reference tasks for the admin registry.
func (s *Server) productTasks(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	bank, err := s.bankFor(slug, filepath.Join(s.productsRoot, slug))
	if err != nil {
		problem(w, http.StatusNotFound, "not_found", "no such product")
		return
	}
	out := make([]map[string]any, 0, len(bank))
	for _, t := range bank {
		out = append(out, map[string]any{"id": t.ID, "kind": t.Kind, "role": t.Role, "title": t.Title, "tags": t.Tags, "check": t.Check})
	}
	writeJSON(w, http.StatusOK, map[string]any{"product": slug, "tasks": out})
}

// sessionReport is the evidence report for a company (or admin) through the
// portal: the same report the admin review shows, with the read audited.
func (s *Server) sessionReport(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.ownSession(w, r)
	if !ok {
		return
	}
	if !s.audit(w, r, sess.ID, "report", "") {
		return
	}
	s.writeReview(w, r, sess, false)
}
