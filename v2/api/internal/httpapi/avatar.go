package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/assistant"
	"github.com/mindfries/v2/api/internal/mw"
	"github.com/mindfries/v2/api/internal/session"
)

// B8: the AI avatar. The browser opens a Gemini Live session directly with a
// short-lived token minted here; what is said comes back as transcripts and is
// kept as evidence beside the terminal assistant's. The avatar is a voice that
// asks; it doesn't judge (assistant.AvatarSystem is locked into the token).

// AvatarStore is the avatar's storage (implemented by store.DB).
type AvatarStore interface {
	AddUtterances(ctx context.Context, sessionID string, us []assistant.Utterance, at time.Time) (int, error)
	AvatarTranscript(ctx context.Context, sessionID string) ([]assistant.Utterance, error)
}

const liveTokenTTL = 40 * time.Minute

func (s *Server) avatarOn(cfgEnabled bool) bool {
	return cfgEnabled && s.cfg.HasGemini() && s.avatar != nil
}

func (s *Server) registerAvatar(mux *http.ServeMux) {
	if s.sessions == nil {
		return
	}
	s.avatarLimit = mw.NewLimiter(6.0/3600, 6) // six Live sessions an hour per candidate is plenty
	mux.HandleFunc("POST /v2/panel/avatar/token", s.panelAuth(s.avatarToken))
	mux.HandleFunc("POST /v2/panel/avatar/transcript", s.panelAuth(s.avatarTranscript))
}

func (s *Server) liveTokens() assistant.LiveTokens {
	return assistant.LiveTokens{APIKey: s.cfg.GeminiKey, Model: s.cfg.GeminiLiveModel, BaseURL: s.cfg.GeminiBaseURL}
}

// avatarToken hands the browser what it needs to talk to Aria.
func (s *Server) avatarToken(w http.ResponseWriter, r *http.Request, sess session.Session) {
	ctx, now := r.Context(), s.now()
	cfg := s.configFor(ctx, sess)
	if !s.avatarOn(cfg.AvatarEnabled) {
		problem(w, http.StatusServiceUnavailable, "not_configured", "the avatar isn't on for this assessment")
		return
	}
	if sess.Status != "active" {
		problemWith(w, http.StatusConflict, "session_not_active", "the avatar talks only while the clock is running",
			map[string]any{"session_status": sess.Status})
		return
	}
	if s.assist != nil {
		if _, left := s.budget(ctx, sess); left <= 0 {
			problem(w, http.StatusTooManyRequests, "budget_exhausted", "this session's assistant budget is used up")
			return
		}
	}
	if ok, _ := s.avatarLimit.Allow(sess.ID); !ok {
		problem(w, http.StatusTooManyRequests, "rate_limited", "too many avatar sessions this hour")
		return
	}
	task, _ := s.taskOf(sess)
	ac := assistant.AvatarContext{
		TaskTitle: firstNonEmpty(task.Title, sess.AssessmentTitle), Brief: sess.Brief, Role: sess.Role,
		ElapsedMin:   int(now.Sub(sess.StartedAt).Minutes()),
		RemainingMin: int(sess.EndsAt.Sub(now).Minutes()),
	}
	if ac.RemainingMin < 0 {
		ac.RemainingMin = 0
	}
	if s.acts != nil {
		if items, err := s.acts.ActivityAfter(ctx, sess.ID, 0, 500); err == nil {
			for i := len(items) - 1; i >= 0 && len(ac.RecentNotes) < 8; i-- {
				if items[i].Severity == activity.Info { // never a flag or a notice: the avatar doesn't accuse
					ac.RecentNotes = append(ac.RecentNotes, items[i].Text)
				}
			}
		}
	}
	ttl := liveTokenTTL
	if left := sess.EndsAt.Sub(now); left < ttl {
		ttl = left
	}
	if ttl < time.Minute {
		problemWith(w, http.StatusConflict, "session_not_active", "there's no time left", map[string]any{"session_status": sess.Status})
		return
	}
	tok, err := s.liveTokens().Mint(ctx, assistant.AvatarSystem(ac), ttl, now)
	if err != nil {
		s.log.Error("avatar: token failed", "session", sess.ID, "err", err)
		if errors.Is(err, assistant.ErrUnavailable) {
			problem(w, http.StatusServiceUnavailable, "internal", "the avatar's voice service is busy; try again in a moment")
		} else {
			problem(w, http.StatusBadGateway, "internal", "couldn't start the avatar")
		}
		return
	}
	s.track(ctx, sess.ID, activity.Item{At: now, Severity: activity.Info, Source: activity.Panel, Category: activity.Observed, Kind: "avatar.start",
		Text: "Started a conversation with " + assistant.AvatarName + ".", DedupeKey: "avatar:" + now.Truncate(10*time.Minute).Format(time.RFC3339)})
	writeJSON(w, http.StatusOK, map[string]any{
		"name": assistant.AvatarName, "token": tok.Token, "model": tok.Model, "ws_url": tok.WSURL, "expires_at": tok.ExpiresAt,
	})
}

// avatarTranscript keeps what was said, as the panel heard it.
func (s *Server) avatarTranscript(w http.ResponseWriter, r *http.Request, sess session.Session) {
	if s.avatar == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "transcripts aren't kept on this server")
		return
	}
	if sess.Status != "active" {
		problemWith(w, http.StatusConflict, "session_not_active", "the session isn't running", map[string]any{"session_status": sess.Status})
		return
	}
	var req struct {
		Utterances []assistant.Utterance `json:"utterances"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 512<<10)).Decode(&req); err != nil || len(req.Utterances) == 0 || len(req.Utterances) > 200 {
		problem(w, http.StatusBadRequest, "malformed", "send 1 to 200 utterances")
		return
	}
	clean := assistant.CleanUtterances(req.Utterances)
	if len(clean) == 0 {
		problem(w, http.StatusBadRequest, "malformed", "no utterance had a seq, a speaker (candidate or avatar) and text")
		return
	}
	ctx, now := r.Context(), s.now()
	stored, err := s.avatar.AddUtterances(ctx, sess.ID, clean, now)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.assist != nil && stored > 0 {
		// The browser talks to Gemini itself, so the server can't see the
		// tokens: count what was said, so the budget still bites.
		in, out := 0, 0
		for _, u := range clean[len(clean)-stored:] {
			if u.Role == "candidate" {
				in += assistant.Cost(u.Text)
			} else {
				out += assistant.Cost(u.Text)
			}
		}
		_ = s.assist.RecordUsage(ctx, "", sess.ID, "avatar", s.cfg.GeminiLiveModel, in, out, now)
	}
	writeJSON(w, http.StatusOK, map[string]any{"stored": stored})
}
