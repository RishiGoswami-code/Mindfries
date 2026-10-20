package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/mindfries/v2/api/internal/session"
)

// B12: a candidate's rights over what V2 holds about them (as V1's settings
// give them): to see it, and to have it deleted. The candidate portal's
// server calls these after the candidate confirms who they are.

// PrivacyStore is what export and deletion need (implemented by store.DB).
type PrivacyStore interface {
	SessionsOfCandidate(ctx context.Context, candidateID string) ([]session.Session, error)
	EvidencePathsOfSession(ctx context.Context, sessionID string) ([]string, error)
	DeleteSession(ctx context.Context, id string) error
	DeleteCandidateConnections(ctx context.Context, candidateID string) error
	LogDeletion(ctx context.Context, sessionID, what, reason string, count int, at time.Time) error
}

func (s *Server) registerPrivacy(mux *http.ServeMux) {
	if s.sessions == nil || s.privacy == nil {
		return
	}
	mux.HandleFunc("GET /v2/portal/candidates/{id}/export", s.portalOnly(s.exportCandidate))
	mux.HandleFunc("POST /v2/portal/candidates/{id}/delete", s.portalOnly(s.deleteCandidate))
}

// exportCandidate is everything V2 holds about a candidate, as data they can
// keep. Media files are listed, not inlined (they're large); each can be
// requested.
func (s *Server) exportCandidate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		problem(w, http.StatusBadRequest, "malformed", "the candidate id must be a UUID")
		return
	}
	ctx := r.Context()
	list, err := s.privacy.SessionsOfCandidate(ctx, id)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	sessions := []map[string]any{}
	for _, sess := range list {
		item := map[string]any{
			"id": sess.ID, "assessment_id": sess.AssessmentID, "role": sess.Role, "status": sess.Status,
			"started_at": sess.StartedAt, "ends_at": sess.EndsAt, "task_version": sess.TaskVersion,
		}
		if s.acts != nil {
			if items, err := s.acts.ActivityAfter(ctx, sess.ID, 0, 5000); err == nil {
				item["activity"] = items
			}
		}
		if s.panelStore != nil {
			if a, err := s.panelStore.Artifacts(ctx, sess.ID); err == nil {
				item["artifacts"] = a
			}
		}
		if s.staff != nil {
			if t, err := s.staff.SessionTranscript(ctx, sess.ID); err == nil {
				item["assistant_transcript"] = t
			}
		}
		if s.monitor != nil {
			if m, err := s.monitor.EvidenceFor(ctx, sess.ID, "", 5000); err == nil {
				item["media"] = m
			}
			if g, err := s.monitor.Gaps(ctx, sess.ID); err == nil {
				item["monitoring_gaps"] = g
			}
		}
		if s.github != nil {
			if repo, ok, _ := s.github.RepoFor(ctx, sess.ID); ok {
				item["repo"] = repoJSON(repo)
			}
		}
		sessions = append(sessions, item)
	}
	out := map[string]any{"candidate_id": id, "exported_at": s.now(), "sessions": sessions}
	if s.github != nil {
		if c, ok, _ := s.github.ConnectionFor(ctx, id); ok {
			out["github"] = map[string]any{"login": c.Login, "connected_at": c.ConnectedAt}
		}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="mindfries-v2-data.json"`)
	writeJSON(w, http.StatusOK, out)
}

// deleteCandidate deletes a candidate's V2 sessions, evidence files, GitHub
// link and sign-ins. The company's report on them, built elsewhere, is not
// this API's to delete. It needs {"confirm": "delete"}: it can't be undone.
func (s *Server) deleteCandidate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		problem(w, http.StatusBadRequest, "malformed", "the candidate id must be a UUID")
		return
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in); err != nil || in.Confirm != "delete" {
		problem(w, http.StatusBadRequest, "malformed", `send {"confirm": "delete"}: this can't be undone`)
		return
	}
	ctx, now := r.Context(), s.now()
	list, err := s.privacy.SessionsOfCandidate(ctx, id)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var files, sessions int
	for _, sess := range list {
		paths, err := s.privacy.EvidencePathsOfSession(ctx, sess.ID)
		if err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		// The bytes go first: if this fails nothing is half-deleted.
		if s.blobs != nil && len(paths) > 0 {
			if err := s.blobs.Delete(ctx, paths); err != nil {
				problem(w, http.StatusBadGateway, "internal", "couldn't delete the stored files; nothing was deleted for this candidate")
				return
			}
		}
		files += len(paths)
		if err := s.privacy.DeleteSession(ctx, sess.ID); err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		sessions++
	}
	if err := s.privacy.DeleteCandidateConnections(ctx, id); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	// The log keeps no candidate or session identity: only that it happened.
	_ = s.privacy.LogDeletion(ctx, "", "candidate_data", "candidate_request", sessions, now)
	writeJSON(w, http.StatusOK, map[string]any{"sessions_deleted": sessions, "files_deleted": files})
}
