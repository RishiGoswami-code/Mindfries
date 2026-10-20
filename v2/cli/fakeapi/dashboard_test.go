package fakeapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/internal/api"
)

func TestDashboardButtonGivesAOneTimeCommand(t *testing.T) {
	s := New(Config{})
	inv := s.Invite(SessionOptions{Title: "Backend", Monitoring: &api.Monitoring{Camera: true, Screen: true, Audio: true}})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	get := func(u string) (int, string) {
		resp, err := http.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	if _, page := get(ts.URL + "/dashboard"); !strings.Contains(page, "Start assessment") || !strings.Contains(page, "monitoring: camera, screen, microphone") {
		t.Fatalf("dashboard:\n%s", page)
	}
	resp, err := http.PostForm(ts.URL+"/dashboard/start", url.Values{"invitation": {inv}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	m := regexp.MustCompile(`/s/([A-Z0-9-]{14})\.ps1 \| iex`).FindStringSubmatch(string(b))
	if m == nil || !strings.Contains(string(b), "curl -fsSL "+ts.URL+"/s/"+m[1]+".sh | sh") {
		t.Fatalf("setup page:\n%s", b)
	}
	code := m[1]

	// The scripts carry the code, and point downloads at this server.
	for _, ext := range []string{"ps1", "sh"} {
		status, body := get(ts.URL + "/s/" + code + "." + ext)
		if status != 200 || !strings.Contains(body, code) || !strings.Contains(body, ts.URL+"/cli/latest") {
			t.Fatalf("%s: %d\n%s", ext, status, body)
		}
	}
	if status, _ := get(ts.URL + "/s/AAAA-BBBB-CCCC.ps1"); status != http.StatusGone {
		t.Fatalf("unknown code: %d", status)
	}

	// setup: the session is ready; begin waits for monitoring.
	c := api.New(ts.URL+Prefix, "0.1.0")
	res, err := c.Exchange(context.Background(), api.ExchangeRequest{SetupCode: code, Device: api.Device{OS: "windows", Arch: "amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Session.Status != api.StatusReady || res.Session.Monitoring == nil || res.Session.DurationMinutes != 90 {
		t.Fatalf("session: %+v", res.Session)
	}
	if status, _ := get(ts.URL + "/s/" + code + ".ps1"); status != http.StatusGone {
		t.Fatalf("used code still served: %d", status)
	}
	c.Token = res.SessionToken
	if _, err := c.Begin(context.Background(), api.BeginRequest{IDE: "vscode"}); !api.HasCode(err, api.CodeMonitoringNotReady) {
		t.Fatalf("began without monitoring: %v", err)
	}
	if _, page := get(ts.URL + "/dashboard/setup?code=" + code); !strings.Contains(page, "Ready to start") {
		t.Fatalf("setup page after setup:\n%s", page)
	}
	if _, page := get(res.Session.Monitoring.PanelURL); !strings.Contains(page, "Share your screen") {
		t.Fatalf("panel:\n%s", page)
	}
	resp, _ = http.Post(res.Session.Monitoring.PanelURL+"/status", "application/json", strings.NewReader(`{"camera":true,"screen":true,"audio":true}`))
	resp.Body.Close()
	sess, err := c.Begin(context.Background(), api.BeginRequest{IDE: "vscode"})
	if err != nil || sess.Status != api.StatusActive || sess.EndsAt.Sub(sess.StartedAt).Minutes() != 90 {
		t.Fatalf("begin: %+v %v", sess, err)
	}
	// A share that stops is a gap the heartbeat reports.
	resp, _ = http.Post(res.Session.Monitoring.PanelURL+"/status", "application/json", strings.NewReader(`{"camera":true,"screen":false,"audio":true}`))
	resp.Body.Close()
	hb, err := c.Heartbeat(context.Background(), api.Heartbeat{})
	if err != nil || hb.Monitoring == nil || hb.Monitoring.Screen || strings.Join(hb.Monitoring.Missing(sess.Monitoring), ",") != "screen" {
		t.Fatalf("heartbeat: %+v %v", hb, err)
	}
}
