package fakeapi

import (
	"net/http"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
)

// The candidate's own GitHub: the session says whether the work goes to a
// repository there (api.GitHub), the CLI shows GitHub's device code on the
// sign-in page (POST /session/github/login) and reports the repository it
// created (POST /session/github/repo). GitHub itself is never faked here;
// tests use fakegithub.

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /session/github/login", s.authed(s.handleGitHubLogin))
		route("POST /session/github/repo", s.authed(s.handleGitHubRepo))
	})
}

// githubFor is the GitHub setting a new session gets: the invitation's,
// else the server's.
func (s *Server) githubFor(o SessionOptions) *api.GitHub {
	g := o.GitHub
	if g == nil {
		g = s.cfg.GitHub
	}
	if g == nil || g.ClientID == "" {
		return nil
	}
	c := *g
	if len(c.Scopes) == 0 {
		c.Scopes = []string{"repo"}
	}
	if c.RepoName == "" {
		c.RepoName = "mindfries-" + slug(o.Title)
	}
	c.Reviewers = append([]string(nil), g.Reviewers...)
	return &c
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(b.String(), "-")
	if out == "" {
		return "assessment"
	}
	if len(out) > 60 {
		out = strings.TrimSuffix(out[:60], "-")
	}
	return out
}

func (s *Server) handleGitHubLogin(w http.ResponseWriter, r *http.Request, ss *session) {
	var in api.GitHubLogin
	if !decode(w, r, &in) || !requireActive(w, ss) {
		return
	}
	if ss.GitHub == nil {
		problem(w, http.StatusConflict, api.CodeMalformed, "This session doesn't use GitHub", "")
		return
	}
	if in.UserCode == "" || !strings.HasPrefix(in.VerificationURI, "https://") && !strings.HasPrefix(in.VerificationURI, "http://") {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "user_code and an http(s) verification_uri are required", "")
		return
	}
	ss.ext.ghLogin = &in
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGitHubRepo(w http.ResponseWriter, r *http.Request, ss *session) {
	var in api.GitHubRepo
	if !decode(w, r, &in) || !requireActive(w, ss) {
		return
	}
	if in.FullName == "" || in.HTMLURL == "" {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "full_name and html_url are required", "")
		return
	}
	ss.ext.ghRepo = &in
	w.WriteHeader(http.StatusNoContent)
}

// GitHubRepo returns the repository a session reported, for tests.
func (s *Server) GitHubRepo(id string) *api.GitHubRepo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil && ss.ext.ghRepo != nil {
		r := *ss.ext.ghRepo
		return &r
	}
	return nil
}
