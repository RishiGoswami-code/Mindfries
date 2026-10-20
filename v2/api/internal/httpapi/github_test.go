package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mindfries/v2/api/internal/config"
	"github.com/mindfries/v2/cli/fakeapi/fakegithub"
)

// githubEnv is an env whose GitHub is the CLI's fake GitHub, so the real
// OAuth round trip runs: authorize, redirect back, code exchange, /user.
func githubEnv(t *testing.T) (*env, *fakegithub.Server) {
	gh := fakegithub.Start(t)
	e := newEnv(t, func(c *config.Config) {
		c.GitHubClientID, c.GitHubClientSecret = "cid", gh.ClientSecret
		c.GitHubWebURL, c.GitHubAPIURL = gh.URL, gh.API
		c.PortalURL = "https://portal.test"
	})
	return e, gh
}

// browse follows a URL through GitHub's redirect and the server's callback
// the way a browser would, without following the final hop out of our hands.
func (e *env) browse(t *testing.T, start string) (final *http.Response) {
	t.Helper()
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Calls to the public base name land on the test server.
		if strings.HasPrefix(req.URL.String(), "https://api.test") {
			u, _ := url.Parse(e.ts.URL + req.URL.RequestURI())
			req.URL = u
			req.Host = ""
		}
		if len(via) > 6 {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	// The first hop is ours: /github/start. Rewrite the public host to the test server.
	if strings.HasPrefix(start, "https://api.test") {
		start = e.ts.URL + strings.TrimPrefix(start, "https://api.test")
	}
	res, err := client.Get(start)
	if err != nil && !strings.Contains(err.Error(), "portal.test") {
		t.Fatal(err)
	}
	if res == nil { // the final redirect pointed at the portal, which isn't running
		return nil
	}
	return res
}

func TestGitHubConnectCollectOnceAndRecordRepo(t *testing.T) {
	e, gh := githubEnv(t)
	tok, sid := e.startSession(newGrant())

	// Nothing connected yet: the CLI is told where to connect.
	r := e.cli("POST", "/session/github/token", tok, map[string]any{})
	if r.Status != 200 || r.str("status") != "pending" || !strings.HasPrefix(r.str("connect_url"), "https://api.test/github/start?s=") {
		t.Fatalf("pending: %d %s", r.Status, r.Raw)
	}
	connect := r.str("connect_url")

	// The candidate opens the link; GitHub authorises and sends them back.
	res := e.browse(t, connect)
	if res == nil || res.StatusCode != 200 || res.Request.URL.Path != "/github/callback" {
		t.Fatalf("callback page: %+v", res)
	}
	res.Body.Close()
	if gh.Authorizations != 1 {
		t.Fatalf("authorize visits: %d", gh.Authorizations)
	}

	// The CLI collects the token, once.
	r = e.cli("POST", "/session/github/token", tok, map[string]any{})
	if r.Status != 200 || r.str("status") != "connected" || r.str("access_token") != gh.Token {
		t.Fatalf("connected: %d %s", r.Status, r.Raw)
	}
	r = e.cli("POST", "/session/github/token", tok, map[string]any{})
	if r.str("status") != "collected" || r.str("access_token") != "" {
		t.Fatalf("the token is handed over once: %s", r.Raw)
	}

	// And the server really forgot it.
	cand := func() string {
		s, _ := e.db.SessionByID(t.Context(), sid)
		return s.Links.CandidateID
	}()
	conn, found, _ := e.db.ConnectionFor(t.Context(), cand)
	if !found || len(conn.Sealed) != 0 || conn.Login != gh.Login {
		t.Fatalf("after collection the connection keeps the login but not the token: %+v", conn)
	}

	// The CLI reports the repository it made.
	repo := map[string]any{
		"login": gh.Login, "full_name": gh.Login + "/mindfries-support-backend",
		"html_url": "https://github.com/" + gh.Login + "/mindfries-support-backend", "default_branch": "main",
		"private": true, "reviewers_invited": []string{"hiring-manager"},
	}
	if r := e.cli("POST", "/session/github/repo", tok, repo); r.Status != http.StatusNoContent {
		t.Fatalf("repo: %d %s", r.Status, r.Raw)
	}
	got, ok, _ := e.db.RepoFor(t.Context(), sid)
	if !ok || got.Owner != gh.Login || len(got.Invited) != 1 {
		t.Fatalf("%+v %v", got, ok)
	}
	repo["private"] = false
	if r := e.cli("POST", "/session/github/repo", tok, repo); r.Status != http.StatusBadRequest {
		t.Fatalf("a public repository is refused: %d", r.Status)
	}
	repo["private"], repo["login"] = true, "someone-else"
	if r := e.cli("POST", "/session/github/repo", tok, repo); r.Status != http.StatusBadRequest {
		t.Fatalf("a repository owned by someone else is refused: %d", r.Status)
	}
}

func TestGitHubDeniedThenRetried(t *testing.T) {
	e, gh := githubEnv(t)
	tok, _ := e.startSession(newGrant())
	connect := e.cli("POST", "/session/github/token", tok, map[string]any{}).str("connect_url")

	gh.SetDeny(true)
	res := e.browse(t, connect)
	if res == nil || res.StatusCode != 200 {
		t.Fatal("the denial page should render")
	}
	res.Body.Close()
	r := e.cli("POST", "/session/github/token", tok, map[string]any{})
	if r.str("status") != "failed" || !strings.Contains(r.str("error"), "didn't authorise") || r.str("connect_url") == "" {
		t.Fatalf("failed: %s", r.Raw)
	}

	gh.SetDeny(false)
	res = e.browse(t, r.str("connect_url"))
	if res == nil {
		t.Fatal("retry")
	}
	res.Body.Close()
	if r := e.cli("POST", "/session/github/token", tok, map[string]any{}); r.str("status") != "connected" {
		t.Fatalf("after a retry: %s", r.Raw)
	}
}

func TestGitHubLinksAndStatesAreSafe(t *testing.T) {
	e, _ := githubEnv(t)
	tok, _ := e.startSession(newGrant())
	// A forged or expired connect link goes nowhere.
	for _, l := range []string{e.ts.URL + "/github/start?s=garbage", e.ts.URL + "/github/start"} {
		res, err := http.Get(l)
		if err != nil || res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: %v %v", l, res, err)
		}
		res.Body.Close()
	}
	// A callback with an unknown state is refused, and a state works once.
	res, _ := http.Get(e.ts.URL + "/github/callback?state=ghs_nope&code=x")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown state: %d", res.StatusCode)
	}
	res.Body.Close()

	connect := e.cli("POST", "/session/github/token", tok, map[string]any{}).str("connect_url")
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, _ = noRedirect.Get(strings.Replace(connect, "https://api.test", e.ts.URL, 1))
	loc, _ := res.Location()
	res.Body.Close()
	if loc == nil || loc.Query().Get("state") == "" {
		t.Fatalf("start should redirect to GitHub with a state: %v", loc)
	}
	back := e.ts.URL + "/github/callback?state=" + loc.Query().Get("state") + "&error=access_denied"
	res, _ = http.Get(back)
	res.Body.Close()
	res, _ = http.Get(back)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("a state is single-use: %d", res.StatusCode)
	}
	res.Body.Close()

	// Without an OAuth app configured, the routes say so.
	off := newEnv(t, nil)
	tok2, _ := off.startSession(newGrant())
	if r := off.cli("POST", "/session/github/token", tok2, map[string]any{}); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("not configured: %d", r.Status)
	}
	if s := off.cli("GET", "/session", tok2, nil).JSON(); s["github"] != nil {
		t.Fatal("a server without GitHub offers no github block")
	}
}

func TestPortalConnectsGitHubFromTheSignInPage(t *testing.T) {
	e, gh := githubEnv(t)
	g := newGrant()
	cand := g["candidate_id"].(string)

	if r := e.portal("GET", "/v2/portal/github/status?candidate_id="+cand, nil); r.JSON()["connected"] != false {
		t.Fatalf("%s", r.Raw)
	}
	r := e.portal("POST", "/v2/portal/github/connect", map[string]any{"candidate_id": cand, "return_to": "https://portal.test/v2/device?code=ABCD-EFGH"})
	if r.Status != 200 || !strings.HasPrefix(r.str("authorize_url"), gh.URL+"/login/oauth/authorize?") {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	// GitHub authorises, the callback redirects back to the portal's page.
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, _ := noRedirect.Get(r.str("authorize_url"))
	cb, _ := res.Location()
	res.Body.Close()
	res, _ = noRedirect.Get(e.ts.URL + cb.RequestURI())
	back, _ := res.Location()
	res.Body.Close()
	if back == nil || back.Host != "portal.test" || back.Path != "/v2/device" || back.Query().Get("github") != "connected" || back.Query().Get("code") != "ABCD-EFGH" {
		t.Fatalf("the browser goes back to the sign-in page: %v", back)
	}
	st := e.portal("GET", "/v2/portal/github/status?candidate_id="+cand, nil).JSON()
	if st["connected"] != true || st["login"] != gh.Login || st["token_waiting"] != true {
		t.Fatalf("%v", st)
	}

	// The session that candidate starts next collects the token without another click.
	tok, _ := e.startSession(g)
	if r := e.cli("POST", "/session/github/token", tok, map[string]any{}); r.str("status") != "connected" || r.str("access_token") != gh.Token {
		t.Fatalf("%s", r.Raw)
	}

	// A return address off the portal is dropped, not followed.
	r = e.portal("POST", "/v2/portal/github/connect", map[string]any{"candidate_id": g["candidate_id"], "return_to": "https://evil.example/steal"})
	res, _ = noRedirect.Get(r.str("authorize_url"))
	cb, _ = res.Location()
	res.Body.Close()
	res, _ = noRedirect.Get(e.ts.URL + cb.RequestURI())
	if loc, _ := res.Location(); loc != nil {
		t.Fatalf("an off-portal return address must not be redirected to: %v", loc)
	}
	res.Body.Close()
}
