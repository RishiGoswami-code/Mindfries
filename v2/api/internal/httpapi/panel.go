package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/monitor"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

// The floating panel's own routes, called from the candidate's browser. The
// browser never holds a session token or a portal secret. The candidate
// portal's server checks the signed-in candidate owns the session, asks for a
// short-lived panel token here, and gives it to the page:
//
//	portal page --(candidate signed in)--> portal server --X-Portal-Secret--> POST /v2/portal/sessions/{id}/panel-token
//	portal page <--panel token (15 min)--- portal server
//	portal page --Bearer panel token--> /v2/panel/*

const panelTokenTTL = 15 * time.Minute

func (s *Server) panelOn() bool { return len(s.cfg.EncryptionKey) == 32 }

func (s *Server) registerPanel(mux *http.ServeMux) {
	if s.sessions == nil {
		return
	}
	mux.HandleFunc("POST /v2/portal/sessions/{id}/panel-token", s.portalOnly(s.portalPanelToken))
	mux.HandleFunc("POST /v2/panel/status", s.panelAuth(s.panelReport))
	mux.HandleFunc("POST /v2/panel/upload/{what}", s.panelAuth(s.panelUpload))
	s.registerPanelData(mux)
}

// panelAuth runs h for a request carrying a valid panel token.
func (s *Server) panelAuth(h func(http.ResponseWriter, *http.Request, session.Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.panelOn() {
			problem(w, http.StatusServiceUnavailable, "not_configured", "the panel isn't set up on this server")
			return
		}
		tok := bearer(r)
		if tok == "" {
			tok = r.URL.Query().Get("token") // EventSource can't set headers
		}
		id, err := auth.CheckPanelToken(s.tokenKey(), tok, []string{auth.ScopePanel}, s.now())
		if err != nil {
			problem(w, http.StatusUnauthorized, "token_invalid", "the panel's session has expired; reload the page")
			return
		}
		sess, err := s.sessions.SessionByID(r.Context(), id)
		if err != nil {
			problem(w, http.StatusUnauthorized, "token_invalid", "unknown session")
			return
		}
		h(w, r, sess)
	}
}

// portalPanelToken is how the candidate portal's server gets a panel token
// for a session, after checking who is signed in. The candidate must be the
// one the session was made for.
func (s *Server) portalPanelToken(w http.ResponseWriter, r *http.Request) {
	if !s.panelOn() {
		problem(w, http.StatusServiceUnavailable, "not_configured", "the panel isn't set up on this server")
		return
	}
	var req struct {
		CandidateID string `json:"candidate_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil || !uuidRe.MatchString(req.CandidateID) {
		problem(w, http.StatusBadRequest, "malformed", "candidate_id must be a UUID")
		return
	}
	sess, err := s.sessions.SessionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		problem(w, http.StatusNotFound, "not_found", "no such session")
		return
	}
	if sess.Links.CandidateID != req.CandidateID {
		problem(w, http.StatusForbidden, "forbidden", "that session belongs to someone else")
		return
	}
	now := s.now()
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      auth.IssuePanelToken(s.tokenKey(), sess.ID, auth.ScopePanel, panelTokenTTL, now),
		"expires_at": now.Add(panelTokenTTL),
		"session_id": sess.ID,
		"status":     sess.Status,
		"ends_at":    sess.EndsAt,
	})
}

// panelReport: the panel says what it's sharing, every few seconds.
func (s *Server) panelReport(w http.ResponseWriter, r *http.Request, sess session.Session) {
	if s.monitor == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "monitoring isn't set up on this server")
		return
	}
	var req struct {
		Camera bool `json:"camera"`
		Screen bool `json:"screen"`
		Audio  bool `json:"audio"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "camera, screen and audio are required")
		return
	}
	ctx, now := r.Context(), s.now()
	cfg := s.configFor(ctx, sess)
	next := monitor.State{SessionID: sess.ID, Camera: req.Camera, Screen: req.Screen, Audio: req.Audio, ReportedAt: now}
	prev, had, err := s.monitor.ReportMonitoring(ctx, next)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	need := requirements(cfg)
	if sess.Status == "active" {
		s.noteShareChanges(ctx, sess, monitor.Changes(prev, had, next, need), now)
		// The panel is back: end a silence gap.
		if g, closed, _ := s.monitor.CloseGap(ctx, sess.ID, monitor.Panel, now); closed {
			s.trackGapEnd(ctx, sess, g, now)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_status": sess.Status,
		"ends_at":        sess.EndsAt,
		"requires":       map[string]bool{"camera": need.Camera, "screen": need.Screen, "audio": need.Audio},
		"missing":        monitor.Missing(need, next, now),
	})
}

// noteShareChanges turns a share stopping or resuming into gaps and
// tracker lines. Wording is plain: what was recorded, not what it means.
func (s *Server) noteShareChanges(ctx context.Context, sess session.Session, changes []monitor.Change, now time.Time) {
	for _, c := range changes {
		if !c.On {
			opened, err := s.monitor.OpenGap(ctx, sess.ID, c.Source, now, c.Source+" sharing stopped")
			if err == nil && opened {
				s.track(ctx, sess.ID, activity.Item{
					At: now, Severity: activity.Notice, Source: activity.Panel, Category: activity.Observed,
					Kind: "gap." + c.Source + ".open", Text: sourceLabel(c.Source) + " sharing stopped. Recorded as a gap.",
					DedupeKey: "gap:" + c.Source + ":" + now.Format(time.RFC3339Nano),
				})
			}
			continue
		}
		if g, closed, _ := s.monitor.CloseGap(ctx, sess.ID, c.Source, now); closed {
			s.trackGapEnd(ctx, sess, g, now)
		}
	}
}

// longGap is how long a share can be off before it's a flag rather than a
// notice: a few seconds of a browser permission prompt is not worth a flag.
const longGap = 30 * time.Second

func (s *Server) trackGapEnd(ctx context.Context, sess session.Session, g store.Gap, now time.Time) {
	d := now.Sub(g.StartedAt).Round(time.Second)
	sev, cat := activity.Notice, activity.Observed
	if d >= longGap {
		sev, cat = activity.Flag, activity.Signal
	}
	s.track(ctx, sess.ID, activity.Item{
		At: now, Severity: sev, Source: activity.Panel, Category: cat, Kind: "gap." + g.Source + ".close",
		Text:      fmt.Sprintf("%s was off for %s. Recorded as a gap.", sourceLabel(g.Source), d),
		DedupeKey: fmt.Sprintf("gap-end:%d", g.ID),
	})
}

func sourceLabel(src string) string {
	switch src {
	case monitor.Camera:
		return "Camera"
	case monitor.Screen:
		return "Screen"
	case monitor.Audio:
		return "Microphone"
	case monitor.Panel:
		return "The monitoring panel"
	}
	return src
}

// panelUpload stores a camera still, a screenshot or a microphone clip. They
// are kept only while the clock runs, only for what this assessment collects,
// and only as often as the panel is meant to send them.
func (s *Server) panelUpload(w http.ResponseWriter, r *http.Request, sess session.Session) {
	if s.monitor == nil || s.blobs == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "evidence storage isn't set up on this server")
		return
	}
	what := r.PathValue("what")
	up, ok := monitor.ForUpload(what, r.Header.Get("Content-Type"))
	if !ok {
		problem(w, http.StatusBadRequest, "malformed", "unsupported upload: "+what+" as "+r.Header.Get("Content-Type"))
		return
	}
	ctx, now := r.Context(), s.now()
	if sess.Status != "active" {
		problemWith(w, http.StatusConflict, "session_not_active", "evidence is kept only while the clock is running",
			map[string]any{"session_status": sess.Status})
		return
	}
	cfg := s.configFor(ctx, sess)
	if (what == "camera" && !cfg.RequireCamera) || (what == "screen" && !cfg.RequireScreen) || (what == "audio" && !cfg.RequireAudio) {
		problem(w, http.StatusForbidden, "forbidden", "this assessment doesn't collect "+what)
		return
	}
	if last, found, err := s.monitor.LastEvidenceAt(ctx, sess.ID, up.Kind); err == nil && found && now.Sub(last) < up.Every {
		w.Header().Set("Retry-After", fmt.Sprint(int(up.Every.Seconds())))
		problem(w, http.StatusTooManyRequests, "rate_limited", "too soon since the last "+what+" upload")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(up.MaxBytes)))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			problem(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("more than %d bytes", up.MaxBytes))
			return
		}
		problem(w, http.StatusBadRequest, "malformed", "couldn't read the upload")
		return
	}
	if len(body) == 0 {
		problem(w, http.StatusBadRequest, "malformed", "empty upload")
		return
	}
	sum := sha256.Sum256(body)
	id := auth.NewID("evi", 10)
	path := fmt.Sprintf("sessions/%s/%s/%d-%s.%s", sess.ID, up.Kind, now.UnixMilli(), id[4:], up.Ext)
	ct := r.Header.Get("Content-Type")
	if err := s.blobs.Put(ctx, path, ct, body); err != nil {
		s.log.Error("evidence: put failed", "session", sess.ID, "err", err)
		problem(w, http.StatusBadGateway, "internal", "couldn't store the upload")
		return
	}
	days := s.cfg.RetentionDays
	if days <= 0 {
		days = 90
	}
	ev := store.Evidence{ID: id, SessionID: sess.ID, Kind: up.Kind, TakenAt: now, Path: path, Bytes: int64(len(body)),
		SHA256: hex.EncodeToString(sum[:]), ContentType: ct, ExpiresAt: now.Add(time.Duration(days) * 24 * time.Hour)}
	if err := s.monitor.SaveEvidence(ctx, ev); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "kind": up.Kind, "bytes": len(body)})
}
