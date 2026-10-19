package fakeapi

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/mindfries/v2/cli/internal/api"
)

// newTest starts the fake and returns it with a client pointed at it.
func newTest(t *testing.T, cfg Config) (*Server, *api.Client) {
	t.Helper()
	s := New(cfg)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, api.New(ts.URL+Prefix, "0.1.0")
}

func TestVersionAndMinimum(t *testing.T) {
	_, c := newTest(t, Config{MinVersion: "0.2.0", LatestVersion: "0.3.0"})
	_, err := c.Version(context.Background())
	if !api.HasCode(err, api.CodeCLITooOld) {
		t.Fatalf("old CLI not refused: %v", err)
	}
	p, _ := api.AsProblem(err)
	if p.Status != 426 || p.MinimumVersion != "0.2.0" {
		t.Fatalf("problem = %+v", p)
	}

	c.CLIVersion = "0.2.0"
	v, err := c.Version(context.Background())
	if err != nil || v.LatestVersion != "0.3.0" {
		t.Fatalf("version: %+v %v", v, err)
	}
}

func TestMissingVersionHeader(t *testing.T) {
	_, c := newTest(t, Config{})
	c.CLIVersion = ""
	if _, err := c.Version(context.Background()); !api.HasCode(err, api.CodeMalformed) {
		t.Fatalf("missing header accepted: %v", err)
	}
}

func TestOffline(t *testing.T) {
	s, c := newTest(t, Config{})
	s.SetOffline(true)
	_, err := c.Version(context.Background())
	if err == nil || !api.IsRetryable(err) {
		t.Fatalf("offline request: %v", err)
	}
	s.SetOffline(false)
	if _, err := c.Version(context.Background()); err != nil {
		t.Fatal(err)
	}
}
