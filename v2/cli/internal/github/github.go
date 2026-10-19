// Package github is the little of GitHub's API the CLI uses, with the
// candidate's own token (which they authorise on the Mindfries sign-in
// page): the signed-in user, creating the assessment's private
// repository, and inviting the reviewers.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to GitHub. Web is github.com, API is api.github.com; tests
// point both at a fake.
type Client struct {
	Web, API string
	Token    string
	HTTP     *http.Client
}

// New is a client for the real GitHub, or the given overrides.
func New(web, apiURL string) *Client {
	if web == "" {
		web = "https://github.com"
	}
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	return &Client{Web: strings.TrimRight(web, "/"), API: strings.TrimRight(apiURL, "/"), HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// APIError is a failed api.github.com request.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("GitHub API %d: %s", e.Status, e.Message) }

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.API+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "mindfries-cli")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &e)
		return &APIError{Status: resp.StatusCode, Message: e.Message}
	}
	if out == nil || len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, out)
}

// User is the signed-in GitHub user.
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

// NoReplyEmail is the address GitHub keeps for the user's commits, so the
// CLI never needs their real email.
func (u *User) NoReplyEmail() string {
	return fmt.Sprintf("%d+%s@users.noreply.github.com", u.ID, u.Login)
}

// CurrentUser is who the token belongs to.
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	var u User
	return &u, c.do(ctx, http.MethodGet, "/user", nil, &u)
}

// Repo is a repository.
type Repo struct {
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// CreateRepo makes a private repository named name, or name-2, name-3…
// if the candidate already has one by that name.
func (c *Client) CreateRepo(ctx context.Context, name, description string) (*Repo, error) {
	for i := 1; i <= 20; i++ {
		n := name
		if i > 1 {
			n = fmt.Sprintf("%s-%d", name, i)
		}
		var r Repo
		err := c.do(ctx, http.MethodPost, "/user/repos", map[string]any{
			"name": n, "description": description, "private": true, "auto_init": false,
		}, &r)
		var ae *APIError
		if errors.As(err, &ae) && ae.Status == http.StatusUnprocessableEntity {
			continue // the name is taken
		}
		if err != nil {
			return nil, err
		}
		return &r, nil
	}
	return nil, fmt.Errorf("couldn't find a free repository name starting with %s", name)
}

// Invite gives a reviewer read access to the repository.
func (c *Client) Invite(ctx context.Context, fullName, user string) error {
	return c.do(ctx, http.MethodPut, "/repos/"+fullName+"/collaborators/"+url.PathEscape(user), map[string]string{"permission": "pull"}, nil)
}
