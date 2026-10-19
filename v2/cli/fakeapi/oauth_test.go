package fakeapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi/fakegithub"
	"github.com/mindfries/v2/cli/internal/api"
)

func TestGitHubInOneClickFromTheSignInPage(t *testing.T) {
	gh := fakegithub.Start(t)
	s := New(Config{SignInInterval: time.Millisecond, GitHub: &api.GitHub{},
		GitHubApp: GitHubApp{ClientID: "Iv1.x", ClientSecret: gh.ClientSecret, Web: gh.URL}})
	inv := s.Invite(SessionOptions{Title: "Backend"})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	c := api.New(ts.URL+Prefix, "0.1.0")
	ctx := context.Background()

	da, _ := c.StartSignIn(ctx, api.DeviceRequest{Device: api.Device{OS: "windows", Arch: "amd64"}})
	// Connect: the browser goes to GitHub and comes back to the page.
	resp, err := http.PostForm(ts.URL+"/device", url.Values{"code": {da.UserCode}, "invitation": {inv}, "action": {"approve"}})
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if gh.Authorizations != 1 || !strings.Contains(string(page), "GitHub connected") || strings.Contains(string(page), "Connect GitHub</a>") {
		t.Fatalf("after Connect (%d authorizations):\n%s", gh.Authorizations, page)
	}
	res, err := c.PollSignIn(ctx, da.DeviceCode)
	if err != nil {
		t.Fatal(err)
	}
	c.Token = res.SessionToken
	// The CLI collects the token once; the server forgets it.
	tok, err := c.GitHubToken(ctx)
	if err != nil || tok.Status != api.GitHubConnected || tok.AccessToken != gh.Token {
		t.Fatalf("collect: %+v %v", tok, err)
	}
	tok, _ = c.GitHubToken(ctx)
	if tok.Status != api.GitHubCollected || tok.AccessToken != "" || tok.ConnectURL == "" {
		t.Fatalf("second collect: %+v", tok)
	}
}

func TestGitHubRefusedShowsOnThePageAndCanBeRetried(t *testing.T) {
	gh := fakegithub.Start(t)
	gh.SetDeny(true)
	s := New(Config{SignInInterval: time.Millisecond, GitHub: &api.GitHub{},
		GitHubApp: GitHubApp{ClientID: "Iv1.x", ClientSecret: gh.ClientSecret, Web: gh.URL}})
	inv := s.Invite(SessionOptions{})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	c := api.New(ts.URL+Prefix, "0.1.0")
	ctx := context.Background()
	da, _ := c.StartSignIn(ctx, api.DeviceRequest{Device: api.Device{OS: "linux", Arch: "amd64"}})
	resp, _ := http.PostForm(ts.URL+"/device", url.Values{"code": {da.UserCode}, "invitation": {inv}, "action": {"approve"}})
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), "didn&#39;t authorise Mindfries") || !strings.Contains(string(page), "Connect GitHub</a>") {
		t.Fatalf("refused page:\n%s", page)
	}
	res, _ := c.PollSignIn(ctx, da.DeviceCode)
	c.Token = res.SessionToken
	tok, _ := c.GitHubToken(ctx)
	if tok.Status != api.GitHubFailed || tok.ConnectURL == "" {
		t.Fatalf("status: %+v", tok)
	}
	// Retrying through the CLI's link works.
	gh.SetDeny(false)
	if resp, err := http.Get(tok.ConnectURL); err == nil {
		resp.Body.Close()
	}
	if tok, _ = c.GitHubToken(ctx); tok.Status != api.GitHubConnected {
		t.Fatalf("after retry: %+v", tok)
	}
}
