package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/fakeapi/fakegithub"
	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/secret"
	"github.com/mindfries/v2/cli/internal/state"
)

// withGitHub points the harness at a fake GitHub and gives sessions a
// GitHub setting.
func withGitHub(t *testing.T, h *harness, g api.GitHub) *fakegithub.Server {
	gh := fakegithub.Start(t)
	h.env["MINDFRIES_GITHUB_URL"] = gh.URL
	h.env["MINDFRIES_GITHUB_API"] = gh.API
	h.srv = fakeapi.New(fakeapi.Config{Now: h.now, SignInInterval: 10 * time.Millisecond, GitHub: &g,
		GitHubApp: fakeapi.GitHubApp{ClientID: "Iv1.mindfriestest", ClientSecret: gh.ClientSecret, Web: gh.URL}})
	ts := httptest.NewServer(h.srv.Handler())
	t.Cleanup(ts.Close)
	h.url = ts.URL + fakeapi.Prefix
	h.env["MINDFRIES_API"] = h.url
	browseGitHub(t, h.srv)
	return gh
}

// browseGitHub plays the candidate's browser: whenever the CLI waits for
// GitHub, it opens the connect link and follows it to GitHub and back.
func browseGitHub(t *testing.T, srv *fakeapi.Server) {
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
			}
			for _, u := range srv.WaitingForGitHub() {
				if resp, err := http.Get(u); err == nil {
					resp.Body.Close()
				}
			}
		}
	}()
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitRepo(dir).Run(context.Background(), args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(out)
}

func TestStartPutsTheWorkInTheCandidatesGitHub(t *testing.T) {
	h := newHarness(t)
	gh := withGitHub(t, h, api.GitHub{Reviewers: []string{"mindfries-reviewer"}, RepoName: "mindfries-rate-limiter", Deployment: true})
	gh.TakeName("mindfries-rate-limiter") // the candidate already has one

	dir := h.started(fakeapi.SessionOptions{Title: "Rate limiter", Task: quickTask()})
	id := h.srv.OnlySessionID()

	full := "octocandidate/mindfries-rate-limiter-2"
	if got := gh.Collaborators(full); got["mindfries-reviewer"] != "pull" {
		t.Fatalf("reviewers: %v (repos %v)", got, gh.Repos())
	}
	base := gitOut(t, dir, "rev-parse", "HEAD")
	if got := gh.Head(full, "main"); got != base {
		t.Fatalf("pushed %q, want the baseline %q", got, base)
	}
	if r := h.srv.GitHubRepo(id); r == nil || r.FullName != full || len(r.ReviewersInvited) != 1 {
		t.Fatalf("reported repo: %+v", r)
	}
	// The candidate's own identity, the remote, and the helper for its host
	// only.
	if got := gitOut(t, dir, "config", "user.email"); got != "4242+octocandidate@users.noreply.github.com" {
		t.Fatalf("user.email = %q", got)
	}
	if got := gitOut(t, dir, "remote", "get-url", "origin"); got != gh.URL+"/git/"+full+".git" {
		t.Fatalf("origin = %q", got)
	}
	key := "credential." + gh.URL + ".helper"
	if got := gitOut(t, dir, "config", "--get-all", key); !strings.Contains(got, "git-credential") {
		t.Fatalf("%s = %q", key, got)
	}
	// The token is in the store, never in the repository.
	l, _ := state.Load(h.home)
	if tok, err := secret.OpenGitHub(h.home, l.GitHubStore).Get(secret.GitHubID(id)); err != nil || tok != gh.Token {
		t.Fatalf("stored token: %q %v", tok, err)
	}
	if cfg, _ := os.ReadFile(filepath.Join(dir, ".git", "config")); strings.Contains(string(cfg), gh.Token) {
		t.Fatal("the token is in .git/config")
	}

	// The credential helper answers git for that host, and nothing else.
	var out strings.Builder
	a := h.app("protocol=http\nhost="+strings.TrimPrefix(gh.URL, "http://")+"\n\n", nil)
	a.Stdout = &out
	if code := a.Run([]string{"git-credential", "get"}); code != 0 || !strings.Contains(out.String(), "password="+gh.Token) {
		t.Fatalf("helper: %d %q", code, out.String())
	}
	out.Reset()
	a = h.app("protocol=https\nhost=example.com\n\n", nil)
	a.Stdout = &out
	if a.Run([]string{"git-credential", "get"}); out.Len() != 0 {
		t.Fatalf("helper answered another host: %q", out.String())
	}

	// Submit pushes the final commit and records the deployment.
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer site.Close()
	writeFile(t, dir, "limiter.py", "def allow(key):\n    return True\n")
	exit, sout := h.run("", "submit", "-m", "Rate limiter", "--yes", "--url", site.URL)
	if exit != 0 {
		t.Fatalf("submit: %d\n%s", exit, sout)
	}
	head := gitOut(t, dir, "rev-parse", "HEAD")
	if got := gh.Head(full, "main"); got != head {
		t.Fatalf("after submit main = %q, want %q\n%s", got, head, sout)
	}
	sub, _ := h.srv.Submission(id)
	if sub == nil || !strings.Contains(sout, "Pushed to") {
		t.Fatalf("submission %+v\n%s", sub, sout)
	}
	// Cleaned up: no GitHub token, no helper; the remote and repo stay.
	if _, err := secret.OpenGitHub(h.home, "file").Get(secret.GitHubID(id)); err == nil {
		t.Fatal("the GitHub token is still stored")
	}
	if got, _ := gitRepo(dir).Run(context.Background(), "config", "--get-all", key); strings.TrimSpace(got) != "" {
		t.Fatalf("helper left behind: %q", got)
	}
	if got := gitOut(t, dir, "remote", "get-url", "origin"); !strings.HasSuffix(got, full+".git") {
		t.Fatalf("origin removed: %q", got)
	}
}

func TestGitHubRefusedKeepsTheSession(t *testing.T) {
	h := newHarness(t)
	gh := withGitHub(t, h, api.GitHub{Required: true})
	// The candidate first presses Cancel on GitHub, then thinks again.
	gh.DenyFirst(1)
	dir := filepath.Join(t.TempDir(), "ws")
	code := h.srv.MintSetupCode(fakeapi.SessionOptions{Task: quickTask()})
	exit, out := h.run("", "start", code, "--dir", dir, "--yes")
	if exit != 0 || !strings.Contains(out, "You didn't authorise Mindfries on GitHub") || !strings.Contains(out, "Signed in to GitHub as @octocandidate") {
		t.Fatalf("refused, then allowed: %d\n%s", exit, out)
	}
	if strings.Count(out, "Connect GitHub:") != 1 {
		t.Fatalf("explained more than once:\n%s", out)
	}
}

func TestGitHubConnectedOnTheSignInPageNeedsNoWaiting(t *testing.T) {
	h := newHarness(t)
	gh := withGitHub(t, h, api.GitHub{})
	inv := h.srv.Invite(fakeapi.SessionOptions{Title: "Backend", Task: quickTask()})
	// The browser: Connect on the page goes straight to GitHub and back.
	go func() {
		for i := 0; i < 500; i++ {
			if codes := h.srv.PendingSignIns(); len(codes) > 0 {
				resp, err := http.PostForm(strings.TrimSuffix(h.url, fakeapi.Prefix)+"/device", url.Values{"code": {codes[0]}, "invitation": {inv}, "action": {"approve"}})
				if err == nil {
					resp.Body.Close()
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	exit, out := h.run("", "setup", "--dir", filepath.Join(t.TempDir(), "ws"), "--yes", "--no-browser")
	if exit != 0 || !strings.Contains(out, "Signed in to GitHub as @octocandidate") {
		t.Fatalf("start: %d\n%s", exit, out)
	}
	// One trip to GitHub's page, on the sign-in page; the CLI didn't need
	// to send the candidate there again or explain anything.
	if gh.Authorizations != 1 || strings.Contains(out, "Waiting for you to authorise") {
		t.Fatalf("authorizations %d\n%s", gh.Authorizations, out)
	}
}

func TestReviewersAddedLaterAreInvited(t *testing.T) {
	h := newHarness(t)
	gh := withGitHub(t, h, api.GitHub{RepoName: "mindfries-backend"})
	inv := h.srv.Invite(fakeapi.SessionOptions{Title: "Backend", Task: quickTask(), Reviewers: []string{"alice"}})
	confirmed := h.confirmInBrowser(inv, true)
	dir := filepath.Join(t.TempDir(), "ws")
	if exit, out := h.run("", "setup", "--dir", dir, "--yes", "--no-browser"); exit != 0 {
		t.Fatalf("start: %d\n%s", exit, out)
	}
	<-confirmed
	full := "octocandidate/mindfries-backend"
	if got := gh.Collaborators(full); len(got) != 1 || got["alice"] != "pull" {
		t.Fatalf("at start: %v", got)
	}

	// The hiring team grows; the next start invites the newcomer.
	if err := h.srv.SetReviewers(inv, []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	if exit, out := h.run("", "start", "--yes"); exit != 0 || !strings.Contains(out, "Invited @bob") {
		t.Fatalf("resume: %d\n%s", exit, out)
	}
	// And submit invites anyone added after that.
	_ = h.srv.SetReviewers(inv, []string{"alice", "bob", "carol"})
	writeFile(t, dir, "notes.md", "done\n")
	if exit, out := h.run("", "submit", "-m", "x", "--yes"); exit != 0 || !strings.Contains(out, "Invited @carol") {
		t.Fatalf("submit: %d\n%s", exit, out)
	}
	if got := gh.Collaborators(full); len(got) != 3 {
		t.Fatalf("finally: %v", got)
	}
}

func TestNoGitHubWhenTheSessionDoesntUseIt(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask()})
	if out, _ := gitRepo(dir).Run(context.Background(), "remote"); strings.TrimSpace(out) != "" {
		t.Fatalf("remotes: %q", out)
	}
}
