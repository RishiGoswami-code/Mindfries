package fakeapi

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
)

// The candidate's own GitHub: the session says whether the work goes to a
// repository there (api.GitHub), the candidate authorises it in one click
// on the sign-in page (oauth.go), and the CLI reports the repository it
// created (POST /session/github/repo). GitHub itself is never faked here;
// tests use fakegithub.

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /session/github/repo", s.authed(s.handleGitHubRepo))
	})
}

// githubFor is the GitHub setting a new session gets: the invitation's,
// else the server's, with the assessment's reviewers.
func (s *Server) githubFor(o SessionOptions) *api.GitHub {
	g := o.GitHub
	if g == nil {
		g = s.cfg.GitHub
	}
	if g == nil || s.cfg.GitHubApp.ClientID == "" {
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
	if o.Reviewers != nil {
		c.Reviewers = append([]string(nil), o.Reviewers...)
	}
	return &c
}

// githubUser is GitHub's rule for usernames.
var githubUser = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]|-(?:[A-Za-z0-9])){0,38}$`)

// CleanReviewers trims, drops a leading @, removes duplicates (GitHub
// usernames ignore case) and rejects names GitHub wouldn't allow.
func CleanReviewers(in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, r := range in {
		r = strings.TrimPrefix(strings.TrimSpace(r), "@")
		if r == "" {
			continue
		}
		if !githubUser.MatchString(r) {
			return nil, fmt.Errorf("%q isn't a GitHub username", r)
		}
		if k := strings.ToLower(r); !seen[k] {
			seen[k] = true
			out = append(out, r)
		}
	}
	return out, nil
}

// SetReviewers replaces an invitation's reviewers. Sessions already
// started from it get them too; the CLI invites the new ones on its next
// start or at submit.
func (s *Server) SetReviewers(invitationID string, reviewers []string) error {
	clean, err := CleanReviewers(reviewers)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var inv *invitation
	for _, i := range s.invites {
		if i.id == invitationID {
			inv = i
		}
	}
	if inv == nil {
		return errNoInvitation
	}
	inv.opts.Reviewers = clean
	for _, ss := range s.sessions {
		if ss.invitation == invitationID && ss.GitHub != nil {
			ss.GitHub.Reviewers = append([]string(nil), clean...)
		}
	}
	return nil
}

func init() {
	registerAdmin(func(s *Server, mux *http.ServeMux) {
		mux.HandleFunc("GET /__admin/invitations", func(w http.ResponseWriter, r *http.Request) {
			type view struct {
				ID        string   `json:"invitation_id"`
				Title     string   `json:"title"`
				GitHub    bool     `json:"github"`
				Reviewers []string `json:"reviewers"`
			}
			s.mu.Lock()
			out := []view{}
			for _, i := range s.invites {
				v := view{ID: i.id, Title: i.opts.Title, Reviewers: []string{}}
				if g := s.githubFor(i.opts); g != nil {
					v.GitHub, v.Reviewers = true, g.Reviewers
				}
				out = append(out, v)
			}
			s.mu.Unlock()
			writeJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("PUT /__admin/invitations/{id}/reviewers", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Reviewers []string `json:"reviewers"`
			}
			if !decode(w, r, &in) {
				return
			}
			if err := s.SetReviewers(r.PathValue("id"), in.Reviewers); err != nil {
				problem(w, http.StatusBadRequest, api.CodeMalformed, sentence(err), "")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
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
