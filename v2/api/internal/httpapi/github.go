package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/crypt"
	"github.com/mindfries/v2/api/internal/ghub"
	"github.com/mindfries/v2/api/internal/store"
)

// B3: the work goes to the candidate's own private GitHub repository, so the
// hiring team can assess deployment too. The server's part is small:
//
//	candidate clicks Connect  -> GitHub's "Authorize Mindfries" page (skipped if
//	                              they did it before) -> /github/callback
//	server trades the one-time code for the token with the OAuth app's secret,
//	seals it, and holds it until the CLI collects it (once), then forgets it
//	CLI creates the repository and invites the reviewers itself, then reports
//	the repository back to /session/github/repo
//
// Reviewers belong to the assessment (policy.Config), not to the server.

// GitHubStore is what the GitHub step stores (implemented by store.DB).
type GitHubStore interface {
	SaveOAuthState(ctx context.Context, st ghub.State) error
	TakeOAuthState(ctx context.Context, state string, now time.Time) (ghub.State, error)
	FinishOAuthState(ctx context.Context, state, result, errText string) error
	LatestOAuthState(ctx context.Context, sessionID string) (ghub.State, bool, error)
	SaveConnection(ctx context.Context, c ghub.Connection) error
	ConnectionFor(ctx context.Context, candidateID string) (ghub.Connection, bool, error)
	WipeToken(ctx context.Context, candidateID string) error
	RecordHandoff(ctx context.Context, sessionID string) (bool, error)
	HandedOff(ctx context.Context, sessionID string) (bool, error)
	SaveRepo(ctx context.Context, r ghub.Repo) error
	RepoFor(ctx context.Context, sessionID string) (ghub.Repo, bool, error)
}

const oauthStateTTL = 15 * time.Minute

func (s *Server) githubOn() bool { return s.github != nil && s.cfg.HasGitHub() }

func (s *Server) ghClient() ghub.Client {
	return ghub.Client{WebURL: s.cfg.GitHubWebURL, APIURL: s.cfg.GitHubAPIURL,
		ClientID: s.cfg.GitHubClientID, ClientSecret: s.cfg.GitHubClientSecret}
}

func (s *Server) tokenKey() []byte { return crypt.Derive(s.cfg.EncryptionKey, "panel-token") }
func (s *Server) sealKey() []byte  { return crypt.Derive(s.cfg.EncryptionKey, "github-token") }

func (s *Server) registerGitHub(mux *http.ServeMux) {
	if s.sessions == nil {
		return
	}
	mux.HandleFunc("POST "+cliPrefix+"/session/github/token", s.cliGitHubToken)
	mux.HandleFunc("POST "+cliPrefix+"/session/github/repo", s.cliGitHubRepo)
	mux.HandleFunc("GET /github/start", s.githubStart)
	mux.HandleFunc("GET /github/callback", s.githubCallback)
	mux.HandleFunc("POST /v2/portal/github/connect", s.portalOnly(s.portalGitHubConnect))
	mux.HandleFunc("GET /v2/portal/github/status", s.portalOnly(s.portalGitHubStatus))
}

func (s *Server) ghNotConfigured(w http.ResponseWriter) {
	problem(w, http.StatusServiceUnavailable, "not_configured", "GitHub isn't set up on this server")
}

// beginOAuth stores a state and returns GitHub's authorize URL.
func (s *Server) beginOAuth(ctx context.Context, r *http.Request, st ghub.State) (string, error) {
	st.State = auth.NewID("ghs", 16)
	st.ExpiresAt = s.now().Add(oauthStateTTL)
	if err := s.github.SaveOAuthState(ctx, st); err != nil {
		return "", err
	}
	return s.ghClient().AuthorizeURL(s.publicBase(r)+"/github/callback", st.State), nil
}

// ---- the CLI ---------------------------------------------------------------

func (s *Server) cliGitHubToken(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
	if !ok {
		return
	}
	if !s.githubOn() {
		s.ghNotConfigured(w)
		return
	}
	ctx := r.Context()
	if done, err := s.github.HandedOff(ctx, sess.ID); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	} else if done {
		writeJSON(w, http.StatusOK, map[string]any{"status": "collected"})
		return
	}
	connect := s.publicBase(r) + "/github/start?s=" + url.QueryEscape(
		auth.IssuePanelToken(s.tokenKey(), sess.ID, auth.ScopeGitHub, oauthStateTTL, s.now()))

	cand := sess.Links.CandidateID
	if cand == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "failed", "error": "this session isn't tied to a candidate", "connect_url": connect})
		return
	}
	conn, found, err := s.github.ConnectionFor(ctx, cand)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if found && len(conn.Sealed) > 0 {
		tok, err := crypt.Open(s.sealKey(), conn.Sealed, []byte(cand))
		if err != nil {
			// Unreadable (key rotated): treat as not connected, ask again.
			writeJSON(w, http.StatusOK, map[string]any{"status": "pending", "connect_url": connect})
			return
		}
		first, err := s.github.RecordHandoff(ctx, sess.ID)
		if err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if !first {
			writeJSON(w, http.StatusOK, map[string]any{"status": "collected"})
			return
		}
		_ = s.github.WipeToken(ctx, cand) // handed over once; the server forgets it
		writeJSON(w, http.StatusOK, map[string]any{"status": "connected", "access_token": string(tok)})
		return
	}
	if st, ok, _ := s.github.LatestOAuthState(ctx, sess.ID); ok && st.Result == "failed" && s.now().Before(st.ExpiresAt.Add(time.Hour)) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "failed", "error": st.Error, "connect_url": connect})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "pending", "connect_url": connect})
}

func (s *Server) cliGitHubRepo(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
	if !ok {
		return
	}
	if !s.githubOn() {
		s.ghNotConfigured(w)
		return
	}
	var body struct {
		Login            string   `json:"login"`
		FullName         string   `json:"full_name"`
		HTMLURL          string   `json:"html_url"`
		DefaultBranch    string   `json:"default_branch"`
		Private          bool     `json:"private"`
		ReviewersInvited []string `json:"reviewers_invited"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid repository report")
		return
	}
	repo, err := ghub.ParseRepo(sess.ID, body.Login, body.FullName, body.HTMLURL, body.DefaultBranch, body.Private, body.ReviewersInvited)
	if err != nil {
		problem(w, http.StatusBadRequest, "malformed", err.Error())
		return
	}
	if err := s.github.SaveRepo(r.Context(), repo); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- the browser -----------------------------------------------------------

// githubStart is the page a connect link opens: it sends the browser to
// GitHub. The link is signed and short-lived, and tied to one session.
func (s *Server) githubStart(w http.ResponseWriter, r *http.Request) {
	if !s.githubOn() {
		s.ghNotConfigured(w)
		return
	}
	sid, err := auth.CheckPanelToken(s.tokenKey(), r.URL.Query().Get("s"), []string{auth.ScopeGitHub}, s.now())
	if err != nil {
		s.githubPage(w, http.StatusBadRequest, "That link has expired. Run mindfries start again.", false)
		return
	}
	sess, err := s.sessions.SessionByID(r.Context(), sid)
	if err != nil || !sess.Live() || sess.Links.CandidateID == "" {
		s.githubPage(w, http.StatusBadRequest, "That link isn't for an active assessment. Run mindfries start again.", false)
		return
	}
	to, err := s.beginOAuth(r.Context(), r, ghub.State{CandidateID: sess.Links.CandidateID, SessionID: sess.ID})
	if err != nil {
		s.githubPage(w, http.StatusInternalServerError, "Something went wrong. Try again.", false)
		return
	}
	http.Redirect(w, r, to, http.StatusFound)
}

// githubCallback is where GitHub sends the browser back.
func (s *Server) githubCallback(w http.ResponseWriter, r *http.Request) {
	if !s.githubOn() {
		s.ghNotConfigured(w)
		return
	}
	q := r.URL.Query()
	ctx := r.Context()
	st, err := s.github.TakeOAuthState(ctx, q.Get("state"), s.now())
	if errors.Is(err, store.ErrStateInvalid) {
		s.githubPage(w, http.StatusBadRequest, "That GitHub sign-in is out of date. Run mindfries start again.", false)
		return
	}
	if err != nil {
		s.githubPage(w, http.StatusInternalServerError, "Something went wrong. Try again.", false)
		return
	}
	fail := func(msg string) {
		_ = s.github.FinishOAuthState(ctx, st.State, "failed", msg)
		s.finishGitHub(w, r, st, false, msg)
	}
	if e := q.Get("error"); e != "" {
		if e == "access_denied" {
			fail("You didn't authorise Mindfries on GitHub")
		} else {
			fail("GitHub said: " + strings.ReplaceAll(e, "_", " "))
		}
		return
	}
	cl := s.ghClient()
	tok, _, err := cl.Exchange(ctx, q.Get("code"), s.publicBase(r)+"/github/callback")
	if err != nil {
		fail(err.Error())
		return
	}
	login, id, err := cl.User(ctx, tok)
	if err != nil {
		fail(err.Error())
		return
	}
	sealed, err := crypt.Seal(s.sealKey(), []byte(tok), []byte(st.CandidateID))
	if err != nil {
		fail("couldn't keep the token safe")
		return
	}
	if err := s.github.SaveConnection(ctx, ghub.Connection{CandidateID: st.CandidateID, Login: login, GitHubID: id, Sealed: sealed, ConnectedAt: s.now()}); err != nil {
		fail("couldn't save the connection")
		return
	}
	_ = s.github.FinishOAuthState(ctx, st.State, "connected", "")
	s.finishGitHub(w, r, st, true, "")
}

// finishGitHub sends the browser on: back to the portal page that began the
// connection, or to a plain page when it began at the CLI's link.
func (s *Server) finishGitHub(w http.ResponseWriter, r *http.Request, st ghub.State, ok bool, msg string) {
	if to := s.safeReturn(st.ReturnTo); to != "" {
		u, _ := url.Parse(to)
		qv := u.Query()
		if ok {
			qv.Set("github", "connected")
		} else {
			qv.Set("github", "failed")
			qv.Set("reason", msg)
		}
		u.RawQuery = qv.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	if ok {
		s.githubPage(w, http.StatusOK, "GitHub is connected. You can close this tab and go back to your terminal.", true)
		return
	}
	s.githubPage(w, http.StatusOK, msg+". Go back to your terminal and run mindfries start to try again.", false)
}

// safeReturn allows a return address only on the candidate portal.
func (s *Server) safeReturn(to string) string {
	if to == "" || s.cfg.PortalURL == "" || !strings.HasPrefix(to, s.cfg.PortalURL+"/") {
		return ""
	}
	return to
}

func (s *Server) githubPage(w http.ResponseWriter, status int, msg string, good bool) {
	title := "Connect GitHub"
	if good {
		title = "GitHub connected"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s · Mindfries</title>
<body style="font:16px system-ui;max-width:32rem;margin:15vh auto;padding:0 1rem;color:#10183A"><h1 style="font-size:1.4rem">%s</h1><p>%s</p></body>`,
		title, title, html.EscapeString(msg))
}

// ---- the portal's server ----------------------------------------------------

// portalGitHubConnect starts a connection from the portal's sign-in page and
// returns where to send the browser.
func (s *Server) portalGitHubConnect(w http.ResponseWriter, r *http.Request) {
	if !s.githubOn() {
		s.ghNotConfigured(w)
		return
	}
	var req struct {
		CandidateID string `json:"candidate_id"`
		ReturnTo    string `json:"return_to"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil || !uuidRe.MatchString(req.CandidateID) {
		problem(w, http.StatusBadRequest, "malformed", "candidate_id must be a UUID")
		return
	}
	to, err := s.beginOAuth(r.Context(), r, ghub.State{CandidateID: req.CandidateID, ReturnTo: s.safeReturn(req.ReturnTo)})
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authorize_url": to})
}

// portalGitHubStatus says whether a candidate has connected GitHub, for the
// sign-in page's "Connected as …".
func (s *Server) portalGitHubStatus(w http.ResponseWriter, r *http.Request) {
	if !s.githubOn() {
		s.ghNotConfigured(w)
		return
	}
	cand := r.URL.Query().Get("candidate_id")
	if !uuidRe.MatchString(cand) {
		problem(w, http.StatusBadRequest, "malformed", "candidate_id must be a UUID")
		return
	}
	conn, found, err := s.github.ConnectionFor(r.Context(), cand)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := map[string]any{"connected": found}
	if found {
		out["login"] = conn.Login
		out["token_waiting"] = len(conn.Sealed) > 0
	}
	writeJSON(w, http.StatusOK, out)
}
