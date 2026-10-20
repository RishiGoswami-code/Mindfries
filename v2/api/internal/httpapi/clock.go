package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/monitor"
	"github.com/mindfries/v2/api/internal/policy"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

// B4: the clock and the monitoring. A session is `ready` once set up. The
// clock starts only when `mindfries start` calls begin, and begin refuses
// until what the assessment requires (camera, screen, microphone) is live in
// the monitoring panel. After that a share that stops is a gap in the record:
// noted in the tracker, never a verdict.

// MonitorStore is the panel's storage (implemented by store.DB).
type MonitorStore interface {
	ReportMonitoring(ctx context.Context, s monitor.State) (prev monitor.State, had bool, err error)
	MonitoringState(ctx context.Context, sessionID string) (monitor.State, bool, error)
	OpenGap(ctx context.Context, sessionID, source string, at time.Time, reason string) (bool, error)
	CloseGap(ctx context.Context, sessionID, source string, at time.Time) (store.Gap, bool, error)
	Gaps(ctx context.Context, sessionID string) ([]store.Gap, error)
	SaveEvidence(ctx context.Context, e store.Evidence) error
	LastEvidenceAt(ctx context.Context, sessionID, kind string) (time.Time, bool, error)
	EvidenceFor(ctx context.Context, sessionID, kind string, limit int) ([]store.Evidence, error)
}

func requirements(c policy.Config) monitor.Require {
	return monitor.Require{Camera: c.RequireCamera, Screen: c.RequireScreen, Audio: c.RequireAudio}
}

// monitoringStatus is the contract's MonitoringStatus.
func (s *Server) monitoringStatus(ctx context.Context, sess session.Session) map[string]any {
	st := monitor.State{SessionID: sess.ID}
	if s.monitor != nil {
		st, _, _ = s.monitor.MonitoringState(ctx, sess.ID)
	}
	now := s.now()
	out := map[string]any{
		"panel_open": st.Open(now),
		"camera":     st.Live(monitor.Camera, now),
		"screen":     st.Live(monitor.Screen, now),
		"audio":      st.Live(monitor.Audio, now),
	}
	if !st.ReportedAt.IsZero() {
		out["last_seen"] = st.ReportedAt
	}
	return out
}

func (s *Server) cliMonitoring(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.monitoringStatus(r.Context(), sess))
}

// cliBegin starts the clock: ready -> active.
func (s *Server) cliBegin(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if sess.Status == "active" { // beginning again answers it as it is
		writeJSON(w, http.StatusOK, s.wireSession(ctx, sess, s.acked(ctx, sess.ID)))
		return
	}
	var req struct {
		IDE    string `json:"ide"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil && err != io.EOF {
		problem(w, http.StatusBadRequest, "malformed", "invalid begin request")
		return
	}
	cfg := s.configFor(ctx, sess)
	now := s.now()
	if !s.consentOK(ctx, sess) {
		problem(w, http.StatusConflict, "monitoring_not_ready", "Not ready: "+consentMissing+".")
		return
	}
	if s.monitor != nil {
		st, _, err := s.monitor.MonitoringState(ctx, sess.ID)
		if err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if missing := monitor.Missing(requirements(cfg), st, now); len(missing) > 0 {
			problem(w, http.StatusConflict, "monitoring_not_ready", monitor.Describe(missing))
			return
		}
	} // a server built without a monitor store (engine tests) has nothing to wait for
	ends := now.Add(time.Duration(sess.DurationMinutes) * time.Minute)
	if err := s.sessions.Begin(ctx, sess.ID, now, ends); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	sess.Status, sess.StartedAt, sess.EndsAt = "active", now, ends

	s.track(ctx, sess.ID, activity.Item{
		At: now, Severity: activity.Info, Source: activity.Server, Category: activity.Verified, Kind: "session.begin",
		Text: fmt.Sprintf("Assessment started. Your time ends in %d minutes.", sess.DurationMinutes), DedupeKey: "begin",
	})
	if req.IDE != "" {
		s.track(ctx, sess.ID, activity.Item{
			At: now, Severity: activity.Info, Source: activity.Watcher, Category: activity.Observed, Kind: "ide.chosen",
			Text: "Working in " + activity.IDEName(req.IDE) + ".", DedupeKey: "ide",
		})
	}
	for _, c := range req.Checks {
		if c.Status == "warn" {
			s.track(ctx, sess.ID, activity.Item{
				At: now, Severity: activity.Notice, Source: activity.Watcher, Category: activity.Observed, Kind: "preflight.warn",
				Text: "Computer check: " + strings.TrimSpace(c.Name+" "+c.Detail), DedupeKey: "preflight:" + c.Name,
			})
		}
	}
	writeJSON(w, http.StatusOK, s.wireSession(ctx, sess, s.acked(ctx, sess.ID)))
}

func (s *Server) acked(ctx context.Context, sessionID string) int64 {
	n, _ := s.sessions.LastAckedSeq(ctx, sessionID)
	return n
}

// cliHeartbeat: the watcher is alive. The reply says whether to stop and what
// the panel has on, and notices when the panel itself has gone quiet.
func (s *Server) cliHeartbeat(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	ctx, now := r.Context(), s.now()
	status, stop := sess.Status, false
	if status == "active" && now.After(sess.EndsAt) {
		_ = s.sessions.SetStatus(ctx, sess.ID, "expired")
		status, stop = "expired", true
	}
	if status != "ready" && status != "active" {
		stop = true // submitted, revoked, expired: the watcher cleans up
	}
	if status == "active" {
		s.noticePanelSilence(ctx, sess, now)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"server_time":    now,
		"session_status": status,
		"ends_at":        sess.EndsAt,
		"stop":           stop,
		"monitoring":     s.monitoringStatus(ctx, sess),
	})
}

// noticePanelSilence opens a "panel" gap when the monitoring panel has not
// reported for a minute during an active session, and says so in the tracker.
func (s *Server) noticePanelSilence(ctx context.Context, sess session.Session, now time.Time) {
	if s.monitor == nil {
		return
	}
	cfg := s.configFor(ctx, sess)
	if req := requirements(cfg); !req.Camera && !req.Screen && !req.Audio {
		return // nothing is monitored for this assessment
	}
	st, had, err := s.monitor.MonitoringState(ctx, sess.ID)
	if err != nil {
		return
	}
	since := sess.StartedAt
	if had && st.ReportedAt.After(since) {
		since = st.ReportedAt
	}
	if now.Sub(since) < monitor.Silent {
		return
	}
	opened, err := s.monitor.OpenGap(ctx, sess.ID, monitor.Panel, since, "the monitoring panel stopped reporting")
	if err == nil && opened {
		s.track(ctx, sess.ID, activity.Item{
			At: now, Severity: activity.Notice, Source: activity.Server, Category: activity.Signal, Kind: "gap.panel.open",
			Text: "The monitoring panel stopped reporting. Recorded as a gap.", DedupeKey: "gap:panel:" + since.Format(time.RFC3339),
		})
	}
}
