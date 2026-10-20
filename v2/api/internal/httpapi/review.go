package httpapi

import (
	"net/http"
	"os"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/manifest"
	"github.com/mindfries/v2/api/internal/report"
	"github.com/mindfries/v2/api/internal/session"
)

// The admin review surface (GRADE step): read-only, token-authed endpoints that
// serve the evidence report a hiring team reads. No code runs here — the
// objective grade is produced out-of-process and read from storage.

func (s *Server) registerReview(mux *http.ServeMux) {
	mux.HandleFunc("GET /v2/admin/reviews", s.adminReviews)
	mux.HandleFunc("GET /v2/admin/reviews/{id}", s.adminReviewDetail)
}

func (s *Server) adminAuthed(w http.ResponseWriter, r *http.Request) bool {
	tok := os.Getenv("V2_ADMIN_TOKEN")
	if tok == "" || r.Header.Get("X-Admin-Token") != tok {
		problem(w, http.StatusForbidden, "forbidden", "valid X-Admin-Token required")
		return false
	}
	return true
}

// adminReviews lists recent sessions with their submission/grade status.
func (s *Server) adminReviews(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthed(w, r) {
		return
	}
	sessions, err := s.sessions.RecentSessions(r.Context(), 50)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(sessions))
	for _, sess := range sessions {
		item := map[string]any{
			"session_id": sess.ID, "assessment_title": sess.AssessmentTitle,
			"role": sess.Role, "product_slug": sess.ProductSlug, "bank_task_id": sess.BankTaskID,
			"status": sess.Status, "started_at": sess.StartedAt, "submitted": false, "graded": false,
		}
		if sub, err := s.sessions.SubmissionBySession(r.Context(), sess.ID); err == nil {
			item["submitted"] = true
			item["submission_id"] = sub.ID
			item["submission_status"] = sub.Status
			if g, err := s.sessions.GradeBySubmission(r.Context(), sub.ID); err == nil {
				item["graded"] = true
				item["passed"] = g.Passed
			}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

// adminReviewDetail returns one session's full evidence report.
func (s *Server) adminReviewDetail(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthed(w, r) {
		return
	}
	sess, err := s.sessions.SessionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		problem(w, http.StatusNotFound, "not_found", "session not found")
		return
	}
	s.writeReview(w, r, sess, true)
}

// writeReview answers with a session's evidence report. The admin gets the raw
// events as well; a company, whose reads are audited, gets the report without
// them (its evidence route has what it needs).
func (s *Server) writeReview(w http.ResponseWriter, r *http.Request, sess session.Session, withEvents bool) {
	events, _ := s.sessions.Events(r.Context(), sess.ID)

	var sub session.Submission
	var subOut map[string]any
	var grade *session.Grade
	if s2, err := s.sessions.SubmissionBySession(r.Context(), sess.ID); err == nil {
		sub = s2
		subOut = map[string]any{
			"id": sub.ID, "status": sub.Status, "head_commit": sub.HeadCommit, "base_commit": sub.BaseCommit,
			"commit_message": sub.CommitMessage, "bundle_bytes": sub.BundleBytes,
			"task_files_intact": sub.TaskFilesIntact, "created_at": sub.CreatedAt, "verified_at": sub.VerifiedAt,
		}
		if g, err := s.sessions.GradeBySubmission(r.Context(), sub.ID); err == nil {
			grade = &g
		}
	}

	rep := report.Build(sess, sub, events, s.roleCoverage(r, sess.ProductSlug, sess.Role), grade).WithContext(s.reportContext(r.Context(), sess))

	out := map[string]any{
		"session": map[string]any{
			"id": sess.ID, "assessment_title": sess.AssessmentTitle, "role": sess.Role,
			"product_slug": sess.ProductSlug, "bank_task_id": sess.BankTaskID, "task_version": sess.TaskVersion,
			"status": sess.Status, "started_at": sess.StartedAt, "ends_at": sess.EndsAt,
			"duration_minutes": sess.DurationMinutes,
		},
		"submission": subOut,
		"report":     rep,
	}
	if withEvents {
		out["events"] = events
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) roleCoverage(r *http.Request, slug, role string) manifest.RoleCoverage {
	products, err := s.reg.Products(r.Context())
	if err != nil {
		return manifest.RoleCoverage{}
	}
	for _, p := range products {
		if p.Slug == slug {
			return p.Manifest.Roles[core.Role(role)]
		}
	}
	return manifest.RoleCoverage{}
}
