package fakeapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

// GitHub in one click: after Connect on the sign-in page, the browser goes
// straight to GitHub's "Authorize Mindfries" page (only the first time:
// GitHub remembers it after that) and comes back to /github/callback. The
// server trades GitHub's one-time code for the candidate's token with the
// OAuth app's secret, and holds it only until the CLI collects it with
// POST /session/github/token. Nothing to type, no device code.

// GitHubApp is the Mindfries OAuth app.
type GitHubApp struct {
	ClientID     string
	ClientSecret string
	// Web is github.com; tests point it at fakegithub.
	Web string
}

func (a GitHubApp) web() string {
	if a.Web != "" {
		return strings.TrimRight(a.Web, "/")
	}
	return "https://github.com"
}

// ghTarget is what an OAuth state stands for: a sign-in still on the page,
// or a session whose CLI asked to connect (again).
type ghTarget struct {
	userCode  string
	sessionID string
	created   time.Time
}

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /session/github/token", s.authed(s.handleGitHubToken))
	})
}

func (s *Server) githubRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /github/start", s.handleGitHubStart)
	mux.HandleFunc("GET /github/callback", s.handleGitHubCallback)
}

// authorizeURL sends the browser to GitHub for target. s.mu is held.
func (s *Server) authorizeURL(r *http.Request, t ghTarget) string {
	state := randomID("ghs_", 16)
	t.created = s.now()
	s.ghStates[state] = t
	q := url.Values{
		"client_id":    {s.cfg.GitHubApp.ClientID},
		"redirect_uri": {s.publicBase(r) + "/github/callback"},
		"scope":        {"repo"},
		"state":        {state},
	}
	return s.cfg.GitHubApp.web() + "/login/oauth/authorize?" + q.Encode()
}

// handleGitHubStart begins (or retries) the GitHub step, for a sign-in on
// the page (?code=) or a session (?s=, the CLI's connect_url).
func (s *Server) handleGitHubStart(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var t ghTarget
	if code := r.URL.Query().Get("code"); code != "" {
		si := s.findLocked(code)
		if si == nil || si.approved == nil {
			renderPage(w, http.StatusNotFound, devicePage{Error: "There's no connected sign-in with that code."})
			return
		}
		t = ghTarget{userCode: si.userCode, sessionID: si.session}
	} else {
		key := r.URL.Query().Get("s")
		for id, ss := range s.sessions {
			if key != "" && ss.ext.ghKey == key {
				t.sessionID = id
			}
		}
		if t.sessionID == "" {
			renderPage(w, http.StatusNotFound, devicePage{Error: "That link isn't for an active assessment. Run 'mindfries start' again."})
			return
		}
	}
	http.Redirect(w, r, s.authorizeURL(r, t), http.StatusFound)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	t, ok := s.ghStates[q.Get("state")]
	delete(s.ghStates, q.Get("state"))
	s.mu.Unlock()
	if !ok || s.now().Sub(t.created) > 15*time.Minute {
		renderPage(w, http.StatusBadRequest, devicePage{Error: "That GitHub sign-in is out of date. Run 'mindfries start' again."})
		return
	}
	var token, failure string
	if e := q.Get("error"); e != "" {
		failure = "GitHub said: " + strings.ReplaceAll(e, "_", " ")
		if e == "access_denied" {
			failure = "You didn't authorise Mindfries on GitHub"
		}
	} else {
		var err error
		token, err = s.exchangeGitHubCode(r, q.Get("code"))
		if err != nil {
			failure = err.Error()
		}
	}
	s.mu.Lock()
	si := s.findLocked(t.userCode)
	if t.sessionID == "" && si != nil {
		t.sessionID = si.session
	}
	if ss := s.sessions[t.sessionID]; ss != nil {
		ss.ext.ghToken, ss.ext.ghErr = token, failure
	} else if si != nil {
		si.ghToken, si.ghErr = token, failure
	}
	s.mu.Unlock()
	if t.userCode != "" {
		http.Redirect(w, r, "/device?code="+url.QueryEscape(t.userCode), http.StatusSeeOther)
		return
	}
	if failure != "" {
		renderPage(w, http.StatusOK, devicePage{Approved: true, Error: failure + ". Run 'mindfries start' to try again."})
		return
	}
	renderPage(w, http.StatusOK, devicePage{Approved: true, Done: "GitHub is connected. Go back to your terminal; you can close this tab."})
}

// exchangeGitHubCode trades the one-time code for the token. It holds no
// lock: it calls GitHub.
func (s *Server) exchangeGitHubCode(r *http.Request, code string) (string, error) {
	app := s.cfg.GitHubApp
	if app.ClientSecret == "" {
		return "", fmt.Errorf("the server has no GitHub client secret (FAKEAPI_GITHUB_CLIENT_SECRET)")
	}
	form := url.Values{
		"client_id":     {app.ClientID},
		"client_secret": {app.ClientSecret},
		"code":          {code},
		"redirect_uri":  {s.publicBase(r) + "/github/callback"},
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, app.web()+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't reach GitHub: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(b, &out); err != nil || resp.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub answered %s", resp.Status)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("GitHub refused the sign-in: %s", strings.TrimSpace(out.Error+" "+out.Description))
	}
	return out.AccessToken, nil
}

// handleGitHubToken hands the token to the CLI, once.
func (s *Server) handleGitHubToken(w http.ResponseWriter, r *http.Request, ss *session) {
	if !requireSetUp(w, ss) {
		return
	}
	if ss.GitHub == nil {
		problem(w, http.StatusConflict, api.CodeMalformed, "This session doesn't use GitHub", "")
		return
	}
	if ss.ext.ghKey == "" {
		ss.ext.ghKey = randomID("", 16)
	}
	connect := s.publicBase(r) + "/github/start?s=" + ss.ext.ghKey
	switch {
	case ss.ext.ghToken != "":
		tok := ss.ext.ghToken
		ss.ext.ghToken, ss.ext.ghCollected = "", true
		writeJSON(w, http.StatusOK, api.GitHubToken{Status: api.GitHubConnected, AccessToken: tok})
	case ss.ext.ghErr != "":
		writeJSON(w, http.StatusOK, api.GitHubToken{Status: api.GitHubFailed, Error: ss.ext.ghErr, ConnectURL: connect})
	case ss.ext.ghCollected:
		writeJSON(w, http.StatusOK, api.GitHubToken{Status: api.GitHubCollected, ConnectURL: connect})
	default:
		ss.ext.ghWaiting = connect
		writeJSON(w, http.StatusOK, api.GitHubToken{Status: api.GitHubPending, ConnectURL: connect})
	}
}

// WaitingForGitHub returns the connect URLs of sessions whose CLI is
// waiting for GitHub, for tests acting as the browser.
func (s *Server) WaitingForGitHub() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, ss := range s.sessions {
		if ss.ext.ghWaiting != "" && ss.ext.ghToken == "" && !ss.ext.ghCollected {
			out = append(out, ss.ext.ghWaiting)
		}
	}
	return out
}
