package cli

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/github"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/secret"
	"github.com/mindfries/v2/cli/internal/state"
)

// The candidate's own GitHub (when the session's GitHub setting says so):
// after the workspace exists, start signs them in with GitHub's device
// login, creates a private repository in their account, invites the
// reviewers and pushes the work. Their later `git push` (or VS Code's
// Sync) works through `mindfries git-credential`, which hands git the
// token from the keychain; the token is never written into the repo, and
// it's deleted when the session ends. Deploying from the repository (AWS,
// Supabase, Render, …) is up to the candidate and part of the task.

func (a *App) githubClient() *github.Client {
	return github.New(a.Getenv("MINDFRIES_GITHUB_URL"), a.Getenv("MINDFRIES_GITHUB_API"))
}

// connectGitHub runs once per session, after the workspace is set up.
func (a *App) connectGitHub(ctx context.Context, l *state.Local, c *api.Client) error {
	g := l.GitHub
	if g == nil || l.Repo != nil {
		return nil
	}
	home, err := a.home()
	if err != nil {
		return err
	}
	gh := a.githubClient()
	fmt.Fprintln(a.Stdout, "\nConnect GitHub: your work goes to a new private repository in your own GitHub")
	fmt.Fprintln(a.Stdout, "account, so the hiring team can see your commits, CI and deployment.")
	dc, err := gh.StartDeviceLogin(ctx, g.ClientID, g.Scopes)
	if err != nil {
		return gitHubFailed(g, fmt.Errorf("couldn't start the GitHub sign-in: %w", err))
	}
	if err := c.ReportGitHubLogin(ctx, api.GitHubLogin{UserCode: dc.UserCode, VerificationURI: dc.VerificationURI, ExpiresAt: time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second).UTC()}); err != nil {
		fmt.Fprintf(a.Stdout, "  (The Mindfries page couldn't be updated: %v.)\n", err)
	}
	fmt.Fprintf(a.Stdout, "  The Mindfries page in your browser shows this code with an Open GitHub button,\n  or go to %s yourself.\n  GitHub code: %s\n\nWaiting for you to authorise Mindfries on GitHub (Ctrl+C to stop; run 'mindfries start' to try again)...\n", dc.VerificationURI, dc.UserCode)

	token, err := a.pollGitHub(ctx, gh, g.ClientID, dc)
	if err != nil {
		return gitHubFailed(g, err)
	}
	store, err := secret.Save(secret.ChooseGitHub(home, a.Getenv), secret.GitHubID(l.SessionID), token)
	if err != nil {
		return fmt.Errorf("couldn't store the GitHub token safely: %w", err)
	}
	l.GitHubStore = store
	gh.Token = token
	user, err := gh.CurrentUser(ctx)
	if err != nil {
		return gitHubFailed(g, fmt.Errorf("couldn't read your GitHub account: %w", err))
	}
	fmt.Fprintf(a.Stdout, "Signed in to GitHub as @%s.\n", user.Login)
	repo, err := gh.CreateRepo(ctx, g.RepoName, "Mindfries assessment: "+titleOf(l))
	if err != nil {
		return gitHubFailed(g, fmt.Errorf("couldn't create the repository: %w", err))
	}
	var invited []string
	for _, rv := range g.Reviewers {
		if err := gh.Invite(ctx, repo.FullName, rv); err != nil {
			fmt.Fprintf(a.Stdout, "  Couldn't invite the reviewer @%s (%v); tell the hiring team.\n", rv, err)
			continue
		}
		invited = append(invited, rv)
	}
	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	l.Repo = &state.Repo{Login: user.Login, FullName: repo.FullName, HTMLURL: repo.HTMLURL, CloneURL: repo.CloneURL, Branch: branch}

	r := &gitx.Repo{Dir: l.Workspace}
	name := user.Name
	if name == "" {
		name = user.Login
	}
	if err := a.configureRemote(ctx, r, l, name, user.NoReplyEmail()); err != nil {
		return err
	}
	if err := state.Save(home, l); err != nil {
		return err
	}
	if err := a.push(ctx, l, token); err != nil {
		fmt.Fprintf(a.Stdout, "The first push didn't go through (%v); 'git push' or 'mindfries submit' will try again.\n", err)
	}
	if err := c.ReportGitHubRepo(ctx, api.GitHubRepo{Login: user.Login, FullName: repo.FullName, HTMLURL: repo.HTMLURL, DefaultBranch: branch, Private: repo.Private, ReviewersInvited: invited}); err != nil {
		fmt.Fprintf(a.Stdout, "  (Couldn't tell Mindfries about the repository: %v.)\n", err)
	}
	fmt.Fprintf(a.Stdout, "Your repository: %s (private", repo.HTMLURL)
	if len(invited) > 0 {
		fmt.Fprintf(a.Stdout, "; the hiring team can read it")
	}
	fmt.Fprintf(a.Stdout, ").\nCommit and 'git push' as you normally would; 'mindfries submit' pushes too.\n")
	if g.Deployment {
		fmt.Fprintln(a.Stdout, "This task asks you to deploy it: connect your hosting (AWS, Supabase, Render, …) to this repository, and give the URL when you submit.")
	}
	return nil
}

// gitHubFailed explains a GitHub step that didn't work. The session is
// kept either way; a required GitHub stops start until it works.
func gitHubFailed(g *api.GitHub, err error) error {
	if g.Required {
		return &ExitError{Code: 1, Err: fmt.Errorf("%v.\nYour session is kept; run 'mindfries start' to connect GitHub again", err)}
	}
	return fmt.Errorf("%v (your work stays on this computer and is handed in with 'mindfries submit')", err)
}

func (a *App) pollGitHub(ctx context.Context, gh *github.Client, clientID string, dc *github.DeviceCode) (string, error) {
	interval := time.Duration(max(dc.Interval, 1)) * time.Second
	if a.tunePoll != nil {
		interval = a.tunePoll(interval)
	}
	deadline := time.Now().Add(time.Duration(max(dc.ExpiresIn, 60)) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
		tok, err := gh.PollDeviceLogin(ctx, clientID, dc.DeviceCode)
		switch {
		case err == nil:
			return tok, nil
		case errors.Is(err, github.ErrPending):
		case errors.Is(err, github.ErrSlowDown):
			interval += 5 * time.Second
		case errors.Is(err, github.ErrExpired), errors.Is(err, github.ErrDenied):
			return "", err
		default:
			// A network blip: keep waiting.
		}
	}
	return "", github.ErrExpired
}

// credentialKey is git's config key for a helper for the repo's host.
func credentialKey(cloneURL string) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("odd repository address %q", cloneURL)
	}
	return "credential." + u.Scheme + "://" + u.Host + ".helper", nil
}

func (a *App) self() string {
	if a.selfPath != "" {
		return a.selfPath
	}
	p, err := os.Executable()
	if err != nil {
		return "mindfries"
	}
	return p
}

// configureRemote points the workspace at the repository: the candidate's
// own identity for their commits, the remote, pushes to the repo's main
// branch, and mindfries as the credential helper for its host only.
func (a *App) configureRemote(ctx context.Context, r *gitx.Repo, l *state.Local, name, email string) error {
	key, err := credentialKey(l.Repo.CloneURL)
	if err != nil {
		return err
	}
	helper := "!\"" + filepath.ToSlash(a.self()) + "\" git-credential"
	if h := a.Getenv("MINDFRIES_HOME"); h != "" {
		helper += " --home \"" + filepath.ToSlash(h) + "\""
	}
	_, _ = r.Run(ctx, "remote", "remove", "origin")
	steps := [][]string{
		{"config", "user.name", name},
		{"config", "user.email", email},
		{"remote", "add", "origin", l.Repo.CloneURL},
		{"config", "push.default", "upstream"},
		// An empty helper first clears any the candidate has for this host.
		{"config", "--replace-all", key, ""},
		{"config", "--add", key, helper},
	}
	for _, s := range steps {
		if _, err := r.Run(ctx, s...); err != nil {
			return fmt.Errorf("couldn't set up the GitHub remote: %w", err)
		}
	}
	return nil
}

// push sends the workspace branch to the repository. It passes the token
// for this one command, so it doesn't depend on the helper.
func (a *App) push(ctx context.Context, l *state.Local, token string) error {
	if l.Repo == nil {
		return nil
	}
	auth := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	key, err := credentialKey(l.Repo.CloneURL)
	if err != nil {
		return err
	}
	// Through the environment, not "-c", so the token never shows in an
	// error message (git errors quote their arguments) or a process list.
	r := &gitx.Repo{Dir: l.Workspace, Env: []string{
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=" + key, "GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_1=http.extraHeader", "GIT_CONFIG_VALUE_1=Authorization: Basic " + auth,
	}}
	_, err = r.Run(ctx, "push", "-q", "-u", "origin", l.Branch+":"+l.Repo.Branch)
	return err
}

// pushForSubmit pushes the final commit, and returns it if that worked.
// A failed push never stops the hand-in: the bundle carries the work.
func (a *App) pushForSubmit(ctx context.Context, l *state.Local) string {
	if l.Repo == nil {
		return ""
	}
	tok, err := a.githubToken(l)
	if err == nil {
		err = a.push(ctx, l, tok)
	}
	if err != nil {
		fmt.Fprintf(a.Stdout, "Couldn't push to %s (%v). Your work is still handed in; push it yourself later if you like.\n", l.Repo.FullName, err)
		return ""
	}
	head, _ := (&gitx.Repo{Dir: l.Workspace}).Head(ctx)
	fmt.Fprintf(a.Stdout, "Pushed to %s.\n", l.Repo.HTMLURL)
	return head
}

// githubToken is the session's GitHub token, if there is one.
func (a *App) githubToken(l *state.Local) (string, error) {
	home, err := a.home()
	if err != nil {
		return "", err
	}
	return secret.OpenGitHub(home, l.GitHubStore).Get(secret.GitHubID(l.SessionID))
}

// forgetGitHub deletes the GitHub token and takes mindfries out of the
// workspace's credential helpers; the repository and remote stay.
func (a *App) forgetGitHub(home string, l *state.Local) {
	if l.Repo == nil && l.GitHubStore == "" {
		return
	}
	if secret.Keychain != nil {
		_ = secret.Keychain.Delete(secret.GitHubID(l.SessionID))
	}
	_ = secret.File{Dir: state.Dir(home), Base: "github-token"}.Delete("")
	if l.Repo != nil && l.Workspace != "" {
		if key, err := credentialKey(l.Repo.CloneURL); err == nil {
			_, _ = (&gitx.Repo{Dir: l.Workspace}).Run(context.Background(), "config", "--unset-all", key)
		}
	}
}

// gitCredential is `mindfries git-credential get`, git's credential
// helper for the assessment repository. Hidden from help.
func (a *App) gitCredential(args []string) error {
	fs := a.newFlags("git-credential get", "Git's credential helper for the assessment repository.")
	home := fs.String("home", "", "the CLI's state folder")
	pos, err := parse(fs, args)
	if err != nil || len(pos) == 0 || pos[0] != "get" {
		return nil // store and erase: nothing to do
	}
	in := map[string]string{}
	sc := bufio.NewScanner(a.Stdin)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			break
		}
		in[k] = v
	}
	if *home != "" {
		old := a.Getenv
		a.Getenv = func(k string) string {
			if k == "MINDFRIES_HOME" {
				return *home
			}
			return old(k)
		}
	}
	h, err := a.home()
	if err != nil {
		return nil
	}
	l, err := state.Load(h)
	if err != nil || l.Repo == nil {
		return nil
	}
	u, err := url.Parse(l.Repo.CloneURL)
	if err != nil || in["host"] != u.Host || (in["protocol"] != "" && in["protocol"] != u.Scheme) {
		return nil
	}
	tok, err := a.githubToken(l)
	if err != nil {
		return nil
	}
	fmt.Fprintf(a.Stdout, "username=x-access-token\npassword=%s\n", tok)
	return nil
}
