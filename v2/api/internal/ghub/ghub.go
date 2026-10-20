// Package ghub is the server's side of "the work goes to the candidate's own
// GitHub": the OAuth round trip that gets the candidate's token, and the
// records of it. The CLI, not the server, creates the repository and invites
// the reviewers using that token; the server only collects the token, hands it
// over once, and records the repository the CLI reports.
package ghub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Connection is a candidate's GitHub link. The token is sealed, and wiped
// once the CLI has collected it.
type Connection struct {
	CandidateID string
	Login       string
	GitHubID    int64
	Sealed      []byte
	ConnectedAt time.Time
}

// State is one OAuth round trip: it carries who it's for through GitHub and
// back, and is single-use.
type State struct {
	State       string
	CandidateID string
	SessionID   string
	ReturnTo    string
	ExpiresAt   time.Time
	Used        bool
	Result      string // "", "connected" or "failed"
	Error       string
}

// Repo is the repository the CLI created for a session.
type Repo struct {
	SessionID string
	Owner     string
	Name      string
	HTMLURL   string
	Private   bool
	Invited   []string
	Branch    string
}

var (
	ownerRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	nameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// ParseRepo checks the CLI's report and returns it as a Repo. A repository
// that isn't private is refused: the work belongs in a private one.
func ParseRepo(sessionID, login, fullName, htmlURL, branch string, private bool, invited []string) (Repo, error) {
	owner, name, ok := strings.Cut(fullName, "/")
	if !ok || !ownerRe.MatchString(owner) || !nameRe.MatchString(name) || name == "." || name == ".." {
		return Repo{}, errors.New("full_name must be owner/name")
	}
	if !strings.EqualFold(login, owner) {
		return Repo{}, errors.New("login and full_name's owner differ")
	}
	u, err := url.Parse(htmlURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return Repo{}, errors.New("html_url must be an https address")
	}
	if !private {
		return Repo{}, errors.New("the repository must be private")
	}
	if branch == "" || len(branch) > 250 {
		return Repo{}, errors.New("default_branch is required")
	}
	if invited == nil {
		invited = []string{}
	}
	return Repo{SessionID: sessionID, Owner: owner, Name: name, HTMLURL: htmlURL, Private: true, Invited: invited, Branch: branch}, nil
}

// Client talks to GitHub (or the test double standing in for it).
type Client struct {
	WebURL       string // https://github.com
	APIURL       string // https://api.github.com
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
}

func (c Client) web() string {
	if c.WebURL != "" {
		return strings.TrimRight(c.WebURL, "/")
	}
	return "https://github.com"
}

func (c Client) api() string {
	if c.APIURL != "" {
		return strings.TrimRight(c.APIURL, "/")
	}
	return "https://api.github.com"
}

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// AuthorizeURL is where to send the browser. GitHub skips its consent page
// for a candidate who authorised Mindfries before.
func (c Client) AuthorizeURL(redirect, state string) string {
	q := url.Values{
		"client_id":    {c.ClientID},
		"redirect_uri": {redirect},
		"scope":        {"repo"},
		"state":        {state},
	}
	return c.web() + "/login/oauth/authorize?" + q.Encode()
}

// Exchange trades GitHub's one-time code for the candidate's token, using the
// OAuth app's secret. The error text is safe to show the candidate.
func (c Client) Exchange(ctx context.Context, code, redirect string) (token, scope string, err error) {
	form := url.Values{"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "code": {code}, "redirect_uri": {redirect}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.web()+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := c.http().Do(req)
	if err != nil {
		return "", "", errors.New("couldn't reach GitHub")
	}
	defer res.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&out); err != nil {
		return "", "", errors.New("GitHub's answer wasn't understood")
	}
	if out.Error != "" || out.AccessToken == "" {
		return "", "", fmt.Errorf("GitHub said: %s", strings.ReplaceAll(orDefault(out.Error, "no token"), "_", " "))
	}
	return out.AccessToken, out.Scope, nil
}

// User reads who the token belongs to.
func (c Client) User(ctx context.Context, token string) (login string, id int64, err error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.api()+"/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := c.http().Do(req)
	if err != nil {
		return "", 0, errors.New("couldn't reach GitHub")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("GitHub refused the token (%d)", res.StatusCode)
	}
	var u struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&u); err != nil || u.Login == "" {
		return "", 0, errors.New("GitHub's answer wasn't understood")
	}
	return u.Login, u.ID, nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
