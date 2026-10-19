// Package fakegithub is a stand-in for the parts of GitHub Mindfries uses,
// for tests: OAuth web sign-in, /user, creating repositories, inviting
// collaborators, and a real Git server (git http-backend over CGI) to push
// to. The OAuth pages are at the server's root, the REST API under /api,
// and repositories under /git.
package fakegithub

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// Server is the fake GitHub.
type Server struct {
	URL   string // the "github.com" root
	API   string // the "api.github.com" root
	Login string // the user who signs in
	Token string
	// ClientSecret is the OAuth app's secret the code exchange needs.
	ClientSecret string
	// Authorizations counts visits to the authorize page.
	Authorizations int

	t     *testing.T
	root  string // bare repositories
	mu    sync.Mutex
	codes map[string]bool // OAuth code → used
	// Deny makes the authorize page refuse, as "Cancel" on GitHub does.
	Deny bool
	// denyFirst refuses that many authorisations, then grants.
	denyFirst int
	repos     map[string]*repo
}

type repo struct {
	collaborators map[string]string
}

// Start runs a fake GitHub for the test.
func Start(t *testing.T) *Server {
	t.Helper()
	s := &Server{t: t, Login: "octocandidate", Token: "gho_fakeTOKEN0123456789", root: t.TempDir(), codes: map[string]bool{}, repos: map[string]*repo{}, ClientSecret: "fake-client-secret"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", s.authorize)
	mux.HandleFunc("POST /login/oauth/access_token", s.accessToken)
	mux.HandleFunc("GET /api/user", s.authed(s.user))
	mux.HandleFunc("POST /api/user/repos", s.authed(s.createRepo))
	mux.HandleFunc("PUT /api/repos/{owner}/{name}/collaborators/{user}", s.authed(s.invite))
	mux.Handle("/git/", s.git())
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	s.URL, s.API = ts.URL, ts.URL+"/api"
	return s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// authorize is GitHub's "Authorize Mindfries" page. The fake grants it at
// once (as GitHub does for an app the user authorised before), or refuses
// when Deny is set, and sends the browser back with a one-time code.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || q.Get("client_id") == "" || back.Host == "" {
		http.Error(w, "bad authorize request", http.StatusBadRequest)
		return
	}
	v := url.Values{"state": {q.Get("state")}}
	s.mu.Lock()
	s.Authorizations++
	if s.Deny || s.denyFirst > 0 {
		if s.denyFirst > 0 {
			s.denyFirst--
		}
		v.Set("error", "access_denied")
	} else {
		code := fmt.Sprintf("code_%d", len(s.codes)+1)
		s.codes[code] = false
		v.Set("code", code)
	}
	s.mu.Unlock()
	back.RawQuery = v.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

// accessToken trades a code for the token; it needs the app's secret, and
// each code works once.
func (s *Server) accessToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	s.mu.Lock()
	defer s.mu.Unlock()
	code := r.PostForm.Get("code")
	used, ok := s.codes[code]
	switch {
	case r.PostForm.Get("client_secret") != s.ClientSecret:
		writeJSON(w, http.StatusOK, map[string]string{"error": "incorrect_client_credentials"})
	case !ok || used:
		writeJSON(w, http.StatusOK, map[string]string{"error": "bad_verification_code"})
	default:
		s.codes[code] = true
		writeJSON(w, http.StatusOK, map[string]string{"access_token": s.Token, "token_type": "bearer", "scope": "repo"})
	}
}

// DenyFirst makes the next n authorisations refuse, as pressing Cancel
// and then thinking again does.
func (s *Server) DenyFirst(n int) {
	s.mu.Lock()
	s.denyFirst = n
	s.mu.Unlock()
}

// SetDeny makes the authorize page refuse (true) or grant (false).
func (s *Server) SetDeny(on bool) {
	s.mu.Lock()
	s.Deny = on
	s.mu.Unlock()
}

func (s *Server) authed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+s.Token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
			return
		}
		h(w, r)
	}
}

func (s *Server) user(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"id": 4242, "login": s.Login, "name": "Octo Candidate"})
}

func (s *Server) repoJSON(full string) map[string]any {
	return map[string]any{
		"full_name": full, "html_url": s.URL + "/" + full, "clone_url": s.URL + "/git/" + full + ".git",
		"default_branch": "main", "private": true, "owner": map[string]string{"login": s.Login},
	}
}

func (s *Server) createRepo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Private bool   `json:"private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "name is required"})
		return
	}
	full := s.Login + "/" + in.Name
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.repos[full]; ok {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"message": "name already exists on this account"})
		return
	}
	dir := filepath.Join(s.root, filepath.FromSlash(full)+".git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", dir).CombinedOutput(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": string(out)})
		return
	}
	_ = exec.Command("git", "-C", dir, "config", "http.receivepack", "true").Run()
	s.repos[full] = &repo{collaborators: map[string]string{}}
	writeJSON(w, http.StatusCreated, s.repoJSON(full))
}

// TakeName makes a repository name already used, to test the fallback.
func (s *Server) TakeName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repos[s.Login+"/"+name] = &repo{collaborators: map[string]string{}}
}

func (s *Server) invite(w http.ResponseWriter, r *http.Request) {
	full := r.PathValue("owner") + "/" + r.PathValue("name")
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := s.repos[full]
	if rp == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
		return
	}
	var in struct {
		Permission string `json:"permission"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	rp.collaborators[r.PathValue("user")] = in.Permission
	w.WriteHeader(http.StatusCreated)
}

// Collaborators returns who was invited to a repository, with what access.
func (s *Server) Collaborators(full string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	if rp := s.repos[full]; rp != nil {
		for k, v := range rp.collaborators {
			out[k] = v
		}
	}
	return out
}

// Repos lists the repositories created.
func (s *Server) Repos() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.repos {
		out = append(out, k)
	}
	return out
}

// Head is the commit a branch points at in a repository, "" if none.
func (s *Server) Head(full, branch string) string {
	dir := filepath.Join(s.root, filepath.FromSlash(full)+".git")
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--verify", "-q", "refs/heads/"+branch).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// git serves the bare repositories with git http-backend, for the token
// as the password (any user name), as GitHub does.
func (s *Server) git() http.Handler {
	execPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		s.t.Fatalf("git --exec-path: %v", err)
	}
	backend := filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend")
	if runtime.GOOS == "windows" {
		backend += ".exe"
	}
	if _, err := os.Stat(backend); err != nil {
		s.t.Skipf("git http-backend isn't available: %v", err)
	}
	h := &cgi.Handler{Path: backend, Root: "/git", Env: []string{"GIT_PROJECT_ROOT=" + s.root, "GIT_HTTP_EXPORT_ALL=1", "REMOTE_USER=git"}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := basicAuth(r)
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(s.Token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="GitHub"`)
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func basicAuth(r *http.Request) (string, string, bool) {
	v, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Basic ")
	if !ok {
		return "", "", false
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", "", false
	}
	u, p, ok := strings.Cut(string(b), ":")
	return u, p, ok
}
