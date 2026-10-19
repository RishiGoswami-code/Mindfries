// Package gitx runs Git in the workspace. Every call disables prompts,
// pagers and colour so the output is stable to parse.
package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Repo is a Git working tree.
type Repo struct {
	Dir string
	// Env is added to the environment of every git call.
	Env []string
}

// Error is a failed git command with its output.
type Error struct {
	Args   []string
	Output string
	Err    error
}

func (e *Error) Error() string {
	out := strings.TrimSpace(e.Output)
	if len(out) > 500 {
		out = out[:500] + "…"
	}
	return fmt.Sprintf("git %s: %v: %s", strings.Join(e.Args, " "), e.Err, out)
}

func (r *Repo) cmd(ctx context.Context, args ...string) *exec.Cmd {
	full := append([]string{"-c", "core.pager=cat", "-c", "color.ui=false", "-c", "core.quotepath=false"}, args...)
	c := exec.CommandContext(ctx, "git", full...)
	c.Dir = r.Dir
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	c.Env = append(c.Env, r.Env...)
	return c
}

// Run runs git and returns stdout.
func (r *Repo) Run(ctx context.Context, args ...string) (string, error) {
	c := r.cmd(ctx, args...)
	var out, errb bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errb
	if err := c.Run(); err != nil {
		return out.String(), &Error{Args: args, Output: errb.String() + out.String(), Err: err}
	}
	return out.String(), nil
}

// Bytes runs git and returns stdout as bytes, unmodified.
func (r *Repo) Bytes(ctx context.Context, args ...string) ([]byte, error) {
	c := r.cmd(ctx, args...)
	var out, errb bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errb
	if err := c.Run(); err != nil {
		return nil, &Error{Args: args, Output: errb.String(), Err: err}
	}
	return out.Bytes(), nil
}

// Line runs git and returns the first line of stdout, trimmed.
func (r *Repo) Line(ctx context.Context, args ...string) (string, error) {
	out, err := r.Run(ctx, args...)
	if err != nil {
		return "", err
	}
	l, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(l), nil
}

// Head is the current commit.
func (r *Repo) Head(ctx context.Context) (string, error) { return r.Line(ctx, "rev-parse", "HEAD") }

// HasCommit reports whether sha exists in the repository.
func (r *Repo) HasCommit(ctx context.Context, sha string) bool {
	_, err := r.Run(ctx, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// Branch is the current branch name.
func (r *Repo) Branch(ctx context.Context) (string, error) {
	return r.Line(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

// ChangedFiles counts files that differ from HEAD, including untracked
// ones (not ignored).
func (r *Repo) ChangedFiles(ctx context.Context) (int, error) {
	out, err := r.Run(ctx, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	return n, nil
}
