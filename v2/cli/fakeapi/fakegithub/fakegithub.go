// Package fakegithub is a stand-in for the parts of GitHub the CLI uses,
// for tests: the device login, /user, creating repositories, inviting
// collaborators, and a real Git server (git http-backend over CGI) to push
// to. Web pages and the device login are at the server's root, the REST
// API under /api, and repositories under /git.
package fakegithub

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
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

	t     *testing.T
	root  string // bare repositories
	mu    sync.Mutex
	codes map[string]bool // device code → approved
	// AutoApprove approves a device login on its first poll, as if the
	// candidate entered the code at once.
	AutoApprove bool
	// Deny makes the next device login fail with access_denied.
	Deny  bool
	repos map[string]*repo
	polls int
}

type repo struct {
	collaborators map[string]string
}

// Start runs a fake GitHub for the test.
func Start(t *testing.T) *Server {
	t.Helper()
	s := &Server{t: t, Login: "octocandidate", Token: "gho_fakeTOKEN0123456789", root: t.TempDir(), codes: map[string]bool{}, repos: map[string]*repo{}, AutoApprove: true}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/device/code", s.deviceCode)
	mux.HandleFunc("POST /login/oauth/access_token", s.accessToken)
	mux.HandleFunc("GET /login/device", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("enter the code")) })
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

func (s *Server) deviceCode(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.PostForm.Get("client_id") == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "incorrect_client_credentials"})
		return
	}
	s.mu.Lock()
	code := "dc_" + string(rune('a'+len(s.codes)))
	s.codes[code] = false
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code": code, "user_code": "WDJB-MJHT", "verification_uri": s.URL + "/login/device",
		"expires_in": 900, "interval": 1,
	})
}

func (s *Server) accessToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.polls++
	code := r.PostForm.Get("device_code")
	approved, ok := s.codes[code]
	switch {
	case !ok:
		writeJSON(w, http.StatusOK, map[string]string{"error": "expired_token"})
	case s.Deny:
		writeJSON(w, http.StatusOK, map[string]string{"error": "access_denied"})
	case approved || s.AutoApprove:
		writeJSON(w, http.StatusOK, map[string]string{"access_token": s.Token, "token_type": "bearer", "scope": r.PostForm.Get("scope")})
	default:
		writeJSON(w, http.StatusOK, map[string]string{"error": "authorization_pending"})
	}
}

// Approve lets every pending device login through.
func (s *Server) Approve() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.codes {
		s.codes[c] = true
	}
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
