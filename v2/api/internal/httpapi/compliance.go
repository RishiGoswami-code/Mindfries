package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mindfries/v2/api/internal/policy"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

// Notice and consent, the audit of who reads evidence, and the candidate's
// answer to a flag (SPEC sections 8 and 16, and the release gate): the
// candidate is told plainly what is and isn't collected and accepts it before
// the clock can start; staff reads of sensitive evidence are logged; and a
// candidate can say in their own words what a flag was.

// ComplianceStore is what these need (implemented by store.DB).
type ComplianceStore interface {
	SaveConsent(ctx context.Context, c store.Consent) error
	ConsentFor(ctx context.Context, candidateID, assessmentID string) (store.Consent, bool, error)
	LogAccess(ctx context.Context, a store.Access) error
	AccessLog(ctx context.Context, sessionID string) ([]store.Access, error)
	RespondToFlag(ctx context.Context, sessionID string, r store.FlagResponse) error
	FlagResponses(ctx context.Context, sessionID string) ([]store.FlagResponse, error)
}

// NoticeVersion is the wording the candidate accepts. Change it when what is
// collected changes, and candidates are asked again.
const NoticeVersion = "2026-10-21"

// NoticeItem is one line of what is (or isn't) collected.
type NoticeItem struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Collected   bool   `json:"collected"`
}

// noticeFor lists what an assessment collects and doesn't, from its settings.
// It is the one source of that wording: the portal shows it, and what the
// candidate accepted is stored against its ids.
func noticeFor(c policy.Config, retentionDays int) []NoticeItem {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	ret := " Kept for " + strconv.Itoa(retentionDays) + " days, then deleted."
	return []NoticeItem{
		{"camera", "Camera", "A still from your camera about every 30 seconds while the clock runs." + ret, c.RequireCamera},
		{"screen", "Screen", "A screenshot of your screen about every 30 seconds while the clock runs. It can include other windows, so close anything private first." + ret, c.RequireScreen},
		{"audio", "Microphone", "Your microphone in short clips while the clock runs." + ret, c.RequireAudio},
		{"activity", "Your work", "Which files you change and how much, your commits, the checks you run, and what you save in the panel (diagram, notes, review comments). Your code is kept with your submission.", true},
		{"assistant", "Assistant and avatar", "What you ask the coding assistant and say to the avatar, and what they answer, is kept for the reviewers." + ret, c.AssistantMode != policy.Off || c.AvatarEnabled},
		{"environment", "Your computer", "A few checks at the start (git, disk space, installed tools) and a look for coding-agent programs and AI editor extensions while you work. Only matches are kept, never a list of your programs.", true},
		{"github", "Your GitHub", "Your work is pushed to a private repository in your own GitHub account that the hiring team can read. The access you grant is used only to create that repository.", true},
		{"not", "What we do not collect", "We do not record your keystrokes, see files outside the assessment folder, or decide anything about you automatically: people review the evidence and make the decision.", false},
	}
}

func parseID(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n > 0
}

func (s *Server) registerCompliance(mux *http.ServeMux) {
	if s.sessions == nil || s.compliance == nil {
		return
	}
	mux.HandleFunc("GET /v2/portal/assessments/{id}/notice", s.portalOnly(s.getNotice))
	mux.HandleFunc("POST /v2/portal/consents", s.portalOnly(s.postConsent))
	mux.HandleFunc("GET /v2/portal/consents", s.portalOnly(s.getConsent))
	mux.HandleFunc("GET /v2/portal/sessions/{id}/access-log", s.staffOnly(s.accessLog))
	mux.HandleFunc("POST /v2/panel/activity/{id}/response", s.panelAuth(s.respondToFlag))
}

func (s *Server) getNotice(w http.ResponseWriter, r *http.Request) {
	cfg := policy.Default()
	if s.configs != nil {
		if c, _, err := s.configs.AssessmentConfig(r.Context(), r.PathValue("id")); err == nil {
			cfg = c
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": NoticeVersion, "items": noticeFor(cfg, s.cfg.RetentionDays)})
}

func (s *Server) postConsent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CandidateID  string   `json:"candidate_id"`
		AssessmentID string   `json:"assessment_id"`
		Version      string   `json:"version"`
		Items        []string `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in); err != nil || !uuidRe.MatchString(in.CandidateID) || in.AssessmentID == "" || len(in.AssessmentID) > 64 {
		problem(w, http.StatusBadRequest, "malformed", "candidate_id (a UUID), assessment_id and version are required")
		return
	}
	if in.Version != NoticeVersion {
		problem(w, http.StatusConflict, "conflict", "the notice has changed; show the candidate the current one ("+NoticeVersion+")")
		return
	}
	// What they accept must include everything this assessment collects.
	cfg := policy.Default()
	if s.configs != nil {
		if c, _, err := s.configs.AssessmentConfig(r.Context(), in.AssessmentID); err == nil {
			cfg = c
		}
	}
	have := map[string]bool{}
	for _, id := range in.Items {
		have[id] = true
	}
	for _, it := range noticeFor(cfg, s.cfg.RetentionDays) {
		if it.Collected && !have[it.ID] {
			problem(w, http.StatusBadRequest, "malformed", "the candidate must accept every collected item; missing: "+it.ID)
			return
		}
	}
	err := s.compliance.SaveConsent(r.Context(), store.Consent{CandidateID: in.CandidateID, AssessmentID: in.AssessmentID,
		Version: in.Version, Items: in.Items, AcceptedAt: s.now()})
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getConsent(w http.ResponseWriter, r *http.Request) {
	cand, asmt := r.URL.Query().Get("candidate_id"), r.URL.Query().Get("assessment_id")
	if !uuidRe.MatchString(cand) || asmt == "" {
		problem(w, http.StatusBadRequest, "malformed", "candidate_id and assessment_id are required")
		return
	}
	c, ok, err := s.compliance.ConsentFor(r.Context(), cand, asmt)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accepted": ok && c.Version == NoticeVersion, "current_version": NoticeVersion, "consent": c})
}

// consentOK reports whether the candidate has accepted the current notice. A
// session with no candidate or assessment on record (an older flow) has none
// to accept.
func (s *Server) consentOK(ctx context.Context, sess session.Session) bool {
	if s.compliance == nil || sess.Links.CandidateID == "" || sess.AssessmentID == "" {
		return true
	}
	c, ok, err := s.compliance.ConsentFor(ctx, sess.Links.CandidateID, sess.AssessmentID)
	return err == nil && ok && c.Version == NoticeVersion
}

const consentMissing = "the candidate hasn't accepted the notice of what is collected"

// audit records who is reading sensitive evidence, as the calling portal says
// (X-Actor), and refuses a read that doesn't name one.
func (s *Server) audit(w http.ResponseWriter, r *http.Request, sessionID, action, detail string) bool {
	if s.compliance == nil {
		return true
	}
	actor := strings.TrimSpace(r.Header.Get("X-Actor"))
	if actor == "" || len(actor) > 200 {
		problem(w, http.StatusBadRequest, "malformed", "X-Actor is required: say who is reading this (for example company:pat@acme.example)")
		return false
	}
	if err := s.compliance.LogAccess(r.Context(), store.Access{SessionID: sessionID, Actor: actor, Action: action, Detail: detail, At: s.now()}); err != nil {
		problem(w, http.StatusInternalServerError, "internal", "couldn't record the access: "+err.Error())
		return false
	}
	return true
}

func (s *Server) accessLog(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.ownSession(w, r)
	if !ok {
		return
	}
	log, err := s.compliance.AccessLog(r.Context(), sess.ID)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": log})
}

// respondToFlag is the candidate's own explanation of a flag or notice.
func (s *Server) respondToFlag(w http.ResponseWriter, r *http.Request, sess session.Session) {
	var in struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&in); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "text is required")
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if in.Text == "" || len(in.Text) > 2000 {
		problem(w, http.StatusBadRequest, "malformed", "say what happened in up to 2000 characters")
		return
	}
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		problem(w, http.StatusBadRequest, "malformed", "bad activity id")
		return
	}
	err := s.compliance.RespondToFlag(r.Context(), sess.ID, store.FlagResponse{ActivityID: id, Text: in.Text, At: s.now()})
	if err == session.ErrNotFound {
		problem(w, http.StatusNotFound, "not_found", "no such flag or notice in your session")
		return
	}
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
