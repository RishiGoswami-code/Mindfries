package e2e

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/fakeapi/fakegithub"
	"github.com/mindfries/v2/cli/internal/api"
)

// The work goes to the candidate's own GitHub: start creates the repo and
// pushes, the candidate's own 'git push' works through mindfries's
// credential helper, and submit pushes the final commit and records the
// deployment URL.
func TestStartPushSubmitWithGitHub(t *testing.T) {
	gh := fakegithub.Start(t)
	s := newSessionWith(t, fakeapi.Config{GitHub: &api.GitHub{ClientID: "Iv1.e2e", Reviewers: []string{"hiring-team"}, Deployment: true}})
	s.env = append(s.env, "MINDFRIES_GITHUB_URL="+gh.URL, "MINDFRIES_GITHUB_API="+gh.API)
	wait := s.start()
	full := "octocandidate/mindfries-e2e-task"
	if gh.Head(full, "main") == "" {
		t.Fatalf("nothing pushed at start; repos %v", gh.Repos())
	}

	// The candidate commits and pushes with plain git, as in VS Code.
	write(t, s.ws, "limiter.py", "def allow(key):\n    return True\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "First go"}, {"push", "-q"}} {
		c := exec.Command("git", args...)
		c.Dir, c.Env = s.ws, append(s.env, "GIT_TERMINAL_PROMPT=0")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, _ := exec.Command("git", "-C", s.ws, "rev-parse", "HEAD").Output()
	if got := gh.Head(full, "main"); got != strings.TrimSpace(string(head)) {
		t.Fatalf("candidate's push: main = %q, want %q", got, head)
	}

	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer site.Close()
	out := s.run("submit", "-m", "Done", "--yes", "--url", site.URL)
	if !strings.Contains(out, "Pushed to") || !strings.Contains(out, "Submitted.") {
		t.Fatalf("submit:\n%s", out)
	}
	wait()
	sub, _ := s.srv.Submission(s.srv.OnlySessionID())
	if sub == nil || sub.Status != api.SubmissionVerified {
		t.Fatalf("submission: %+v", sub)
	}
	s.noCredentialsLeft()
}
