package fakeapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/install"
	"github.com/mindfries/v2/cli/internal/api"
)

func startSignIn(t *testing.T, c *api.Client) *api.DeviceAuthorization {
	t.Helper()
	da, err := c.StartSignIn(context.Background(), api.DeviceRequest{Device: api.Device{OS: "windows", Arch: "amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	return da
}

func TestSignInThroughThePage(t *testing.T) {
	s := New(Config{SignInInterval: time.Millisecond})
	inv := s.Invite(SessionOptions{Title: "Rate limiter"})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	c := api.New(ts.URL+Prefix, "0.1.0")

	da := startSignIn(t, c)
	if da.UserCode == "" || len(da.DeviceCode) < 32 || da.IntervalSeconds < 1 {
		t.Fatalf("authorization: %+v", da)
	}
	if da.VerificationURIComplete != ts.URL+"/device?code="+da.UserCode {
		t.Fatalf("link: %s", da.VerificationURIComplete)
	}
	if _, err := c.PollSignIn(context.Background(), da.DeviceCode); !api.HasCode(err, api.CodeAuthorizationPending) {
		t.Fatalf("before confirming: %v", err)
	}

	// The page shows the code, the device and the invitation.
	resp, err := http.Get(da.VerificationURIComplete)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	for _, want := range []string{da.UserCode, "windows/amd64", "Rate limiter", `value="` + inv + `"`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("page lacks %q", want)
		}
	}

	// Confirming it there hands the CLI a session token.
	resp, err = http.PostForm(ts.URL+"/device", url.Values{"code": {strings.ToLower(da.UserCode)}, "email": {"a@b.c"}, "invitation": {inv}, "action": {"approve"}})
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(page), "Connected to <strong>Rate limiter") || !strings.Contains(string(page), "Finishing in your terminal") {
		t.Fatalf("confirm: %d %s", resp.StatusCode, page)
	}
	res, err := c.PollSignIn(context.Background(), da.DeviceCode)
	if err != nil || res.SessionToken == "" || res.Session.AssessmentTitle != "Rate limiter" {
		t.Fatalf("token: %+v %v", res, err)
	}
	// Once the terminal has the session, the page says so.
	resp, err = http.Get(da.VerificationURIComplete)
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), "Go back to your terminal") {
		t.Fatalf("after the terminal collected it:\n%s", page)
	}
	c.Token = res.SessionToken
	if _, err := c.Session(context.Background()); err != nil {
		t.Fatalf("the token doesn't work: %v", err)
	}
	if _, err := c.PollSignIn(context.Background(), da.DeviceCode); !api.HasCode(err, api.CodeDeviceCodeUsed) {
		t.Fatalf("second poll: %v", err)
	}
}

func TestSignInRefusedExpiredAndTooFast(t *testing.T) {
	now := time.Now()
	s := New(Config{Now: func() time.Time { return now }, SignInInterval: 5 * time.Second})
	s.Invite(SessionOptions{})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	c := api.New(ts.URL+Prefix, "0.1.0")
	ctx := context.Background()

	da := startSignIn(t, c)
	if err := s.DenySignIn(da.UserCode); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PollSignIn(ctx, da.DeviceCode); !api.HasCode(err, api.CodeAccessDenied) {
		t.Fatalf("denied: %v", err)
	}

	da = startSignIn(t, c)
	if _, err := c.PollSignIn(ctx, da.DeviceCode); !api.HasCode(err, api.CodeAuthorizationPending) {
		t.Fatalf("first poll: %v", err)
	}
	if _, err := c.PollSignIn(ctx, da.DeviceCode); !api.HasCode(err, api.CodeSlowDown) {
		t.Fatalf("immediate second poll: %v", err)
	}
	now = now.Add(11 * time.Minute)
	if _, err := c.PollSignIn(ctx, da.DeviceCode); !api.HasCode(err, api.CodeDeviceCodeExpired) {
		t.Fatalf("expired: %v", err)
	}
	if err := s.ApproveSignIn(da.UserCode, "", ""); err == nil {
		t.Fatal("approved an expired sign-in")
	}
	if _, err := c.PollSignIn(ctx, "mfd_nope"); !api.HasCode(err, api.CodeDeviceCodeInvalid) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestSignInPageWithoutInvitations(t *testing.T) {
	s, c := newTest(t, Config{})
	da := startSignIn(t, c)
	if err := s.ApproveSignIn(da.UserCode, "", ""); err == nil {
		t.Fatal("approved with no invitation")
	}
	resp, err := http.Get(da.VerificationURIComplete)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), "haven&#39;t been invited") {
		t.Fatalf("page:\n%s", page)
	}
}

func TestHostsInstallersAndBuilds(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "mindfries_windows_amd64.exe"), []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(Config{DistDir: dist})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	get := func(path string) (int, string) {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	for _, p := range []string{"/install.ps1", "/install.sh"} {
		code, body := get(p)
		if code != 200 || !strings.Contains(body, ts.URL+"/cli/latest") || strings.Contains(body, install.DefaultDownload) {
			t.Errorf("%s: %d, download address not rewritten", p, code)
		}
	}
	if code, body := get("/cli/latest/checksums.txt"); code != 200 || !strings.Contains(body, "  mindfries_windows_amd64.exe") {
		t.Errorf("checksums: %d %q", code, body)
	}
	if code, body := get("/cli/latest/mindfries_windows_amd64.exe"); code != 200 || body != "binary" {
		t.Errorf("build: %d %q", code, body)
	}
	if code, _ := get("/cli/latest/..%2fsecret"); code != 404 {
		t.Errorf("odd name served: %d", code)
	}
}
