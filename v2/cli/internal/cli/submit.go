package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
	"github.com/mindfries/v2/cli/internal/workspace"
)

// checkTimeout bounds each of the task's check commands.
var checkTimeout = 10 * time.Minute

func (a *App) submit(args []string) error {
	fs := a.newFlags("submit [flags]", "Runs the task's checks, shows your changes, commits them and hands in.")
	message := fs.String("m", "", "commit message (default: asks)")
	yes := fs.Bool("yes", false, "don't ask: use the defaults and hand in")
	deployURL := fs.String("url", "", "where you deployed it, if the task asks (default: asks)")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	l, c, err := a.active()
	if err != nil {
		return err
	}
	if l.Workspace == "" || l.BaseCommit == "" {
		return errors.New("the workspace isn't set up yet: run 'mindfries start' first")
	}
	home, err := a.home()
	if err != nil {
		return err
	}
	stateDir := state.Dir(home)
	ctx := context.Background()
	u := a.ui(*yes)
	r := &gitx.Repo{Dir: l.Workspace}

	sess, err := c.Session(ctx)
	if err != nil {
		if api.IsAuthFailure(err) {
			a.forget(l)
			return sessionEnded(err)
		}
		return fmt.Errorf("couldn't reach Mindfries to hand in: %w\nYour work is safe; try again when you're back online", err)
	}
	if sess.Status != api.StatusActive {
		return fmt.Errorf("this assessment is %s, so it can't be submitted", sess.Status)
	}

	// The task file must be the task as given.
	for _, lf := range l.Locked {
		if !workspace.LockedIntact(l.Workspace, lf) {
			if err := workspace.RestoreLocked(l.Workspace, stateDir, lf); err != nil {
				return fmt.Errorf("%s was changed and couldn't be put back: %w", lf.Path, err)
			}
			fmt.Fprintf(a.Stdout, "%s had been changed; it was put back.\n", lf.Path)
		}
	}

	// Checks run before staging, so their caches aren't committed.
	checks := a.runChecks(ctx, l, stateDir)

	if _, err := r.Run(ctx, "add", "-A"); err != nil {
		return err
	}
	stat, _ := r.Run(ctx, "diff", "--cached", "--stat", l.BaseCommit)
	if strings.TrimSpace(stat) == "" {
		fmt.Fprintln(a.Stdout, "\nNothing has changed since the start of the assessment.")
		if !u.Confirm("Submit anyway?", false) {
			return &ExitError{Code: 1}
		}
	} else {
		fmt.Fprintf(a.Stdout, "\nYour changes since the start:\n%s\n", stat)
		if !*yes && u.Confirm("Show the full diff?", false) {
			full, _ := r.Run(ctx, "diff", "--cached", l.BaseCommit)
			fmt.Fprintln(a.Stdout, full)
		}
	}
	msg := strings.TrimSpace(*message)
	if msg == "" {
		msg = u.Ask("Commit message", "Submit "+titleOf(l))
	}
	if !*yes && !u.Explicit("Hand in now? You can't change your submission afterwards.") {
		fmt.Fprintln(a.Stdout, "Not submitted. Your changes are staged; carry on and run 'mindfries submit' when you're ready.")
		return nil
	}

	if staged, _ := r.Run(ctx, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		if _, err := r.Run(ctx, "commit", "-q", "--no-verify", "-m", msg); err != nil {
			return err
		}
	}
	if sess.GitHub != nil {
		a.syncReviewers(ctx, l, sess.GitHub.Reviewers)
	}
	pushed := a.pushForSubmit(ctx, l)
	deployment := strings.TrimSpace(*deployURL)
	if deployment == "" && l.GitHub != nil && l.GitHub.Deployment && !*yes {
		deployment = strings.TrimSpace(u.Ask("Where is it deployed? (the URL, or Enter if it isn't)", ""))
	}

	// Let the watcher send everything it has before the session closes.
	if watcher.Running(stateDir) {
		fmt.Fprintln(a.Stdout, "Sending the last of the session record…")
		_ = watcher.RequestStop(stateDir, watcher.ReasonSubmitted)
		watcher.WaitStopped(stateDir, 30*time.Second)
	}
	if q, err := watcher.OpenQueue(stateDir); err == nil && q.Depth() > 0 {
		if left := watcher.Flush(c, q, l.Limits, 20*time.Second); left > 0 {
			fmt.Fprintf(a.Stdout, "%d session events couldn't be sent yet; they'll be noted as a gap.\n", left)
		}
	}

	m, bundlePath, err := a.buildSubmission(ctx, l, stateDir, msg, checks)
	if err != nil {
		return err
	}
	if l.Repo != nil {
		m.GitHubRepo = l.Repo.FullName
		m.PushedCommit = pushed
	}
	m.DeploymentURL = deployment
	if l.Limits.MaxBundleBytes > 0 && m.BundleBytes > l.Limits.MaxBundleBytes {
		return fmt.Errorf("your work is %d MB, more than the %d MB limit. Remove large generated files (add them to .gitignore), then run 'mindfries submit' again", m.BundleBytes>>20, l.Limits.MaxBundleBytes>>20)
	}
	sub, err := c.CreateSubmission(ctx, *m)
	if err != nil {
		return fmt.Errorf("couldn't hand in: %w\nYour work is committed and safe; run 'mindfries submit' again", err)
	}
	f, err := os.Open(bundlePath)
	if err != nil {
		return err
	}
	sub, err = c.UploadBundle(ctx, sub.ID, f, m.BundleBytes)
	f.Close()
	if err != nil {
		if p, ok := api.AsProblem(err); ok && p.Code == api.CodeBundleMismatch {
			fmt.Fprintln(a.Stdout, "The server's copy didn't match what was sent:")
			for _, mm := range p.Mismatches {
				fmt.Fprintf(a.Stdout, "  %s %s\n", mm.What, mm.Path)
			}
			return errors.New("not submitted. Run 'mindfries submit' again; if it keeps happening, tell the hiring team")
		}
		return fmt.Errorf("couldn't upload your work: %w\nIt's committed and safe; run 'mindfries submit' again", err)
	}
	_ = os.Remove(bundlePath)

	fmt.Fprintf(a.Stdout, "\nSubmitted. Commit %s, received %s.\n", short(m.HeadCommit), time.Now().Format("15:04"))
	if m.DeploymentURL != "" {
		if sub.DeploymentStatus > 0 {
			fmt.Fprintf(a.Stdout, "Your deployment at %s answered %d.\n", m.DeploymentURL, sub.DeploymentStatus)
		} else {
			fmt.Fprintf(a.Stdout, "Mindfries couldn't reach %s yet; it's recorded and will be checked again.\n", m.DeploymentURL)
		}
	}
	if l.Repo != nil {
		fmt.Fprintf(a.Stdout, "Your repository stays yours: %s\n", l.Repo.HTMLURL)
	}
	_ = state.SaveLast(home, &state.Last{Title: l.Title, SubmittedAt: time.Now().UTC(), HeadCommit: m.HeadCommit, SubmissionID: sub.ID, Workspace: l.Workspace})
	a.finish(l, c, "submitted")
	fmt.Fprintf(a.Stdout, "This computer's session has been cleaned up. Your code stays in %s.\n", l.Workspace)
	return nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// runChecks runs the task's tests and builds, reports each result and puts
// it in the session record.
func (a *App) runChecks(ctx context.Context, l *state.Local, stateDir string) []api.CheckResult {
	var out []api.CheckResult
	for _, cmd := range l.CheckCommands {
		run := a.runCheck(ctx, l, cmd)
		switch {
		case run.Err != nil:
			fmt.Fprintf(a.Stdout, "Couldn't run it (%v). That's recorded; it doesn't stop you handing in.\n", run.Err)
		case run.TimedOut:
			fmt.Fprintln(a.Stdout, "It timed out. That's recorded; it doesn't stop you handing in.")
		case run.Result.ExitCode != 0:
			fmt.Fprintf(a.Stdout, "It failed (exit code %d). That's recorded; it doesn't stop you handing in.\n", run.Result.ExitCode)
		}
		a.recordCheck(stateDir, run, true)
		out = append(out, run.Result)
	}
	return out
}

// tail keeps the last n lines of s, at most max bytes.
func tail(s string, n, max int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := strings.Join(lines, "\n")
	if len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

// buildSubmission makes the manifest and the Git bundle.
func (a *App) buildSubmission(ctx context.Context, l *state.Local, stateDir, msg string, checks []api.CheckResult) (*api.SubmissionManifest, string, error) {
	r := &gitx.Repo{Dir: l.Workspace}
	head, err := r.Head(ctx)
	if err != nil {
		return nil, "", err
	}
	m := &api.SubmissionManifest{
		TaskVersion: l.TaskVersion, Branch: l.Branch, BaseCommit: l.BaseCommit, HeadCommit: head,
		CommitMessage: msg, Checks: checks, CreatedAt: time.Now().UTC(), TaskFilesIntact: true,
	}
	ns, err := r.Run(ctx, "diff", "--name-status", "--no-renames", l.BaseCommit, head)
	if err != nil {
		return nil, "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(ns), "\n") {
		status, p, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		f := api.SubmissionFile{Path: p}
		switch status[:1] {
		case "A":
			f.Change = "added"
		case "D":
			f.Change = "deleted"
		default:
			f.Change = "modified"
		}
		if f.Change != "deleted" {
			blob, err := r.Bytes(ctx, "cat-file", "blob", head+":"+p)
			if err != nil {
				return nil, "", err
			}
			f.SHA256 = workspace.HashBytes(blob)
		}
		m.Files = append(m.Files, f)
	}
	for _, lf := range l.Locked {
		blob, err := r.Bytes(ctx, "cat-file", "blob", head+":"+lf.Path)
		if err != nil || workspace.HashBytes(blob) != lf.SHA256 {
			m.TaskFilesIntact = false
		}
	}
	bundle := filepath.Join(stateDir, "submission.bundle")
	_ = os.Remove(bundle)
	if _, err := r.Run(ctx, "bundle", "create", bundle, l.Branch); err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(bundle)
	if err != nil {
		return nil, "", err
	}
	m.BundleSHA256, m.BundleBytes = workspace.HashBytes(b), int64(len(b))
	return m, bundle, nil
}
