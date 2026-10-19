package fakeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

func TestReviewersBelongToTheAssessment(t *testing.T) {
	s := New(Config{SignInInterval: time.Millisecond, GitHub: &api.GitHub{ClientID: "Iv1.x"}})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	admin := func(method, path, body string) *http.Response {
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	resp := admin("POST", "/__admin/invitations", `{"title":"Backend","reviewers":["@alice"," Bob ","alice","ALICE"]}`)
	var created struct {
		ID string `json:"invitation_id"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	if resp := admin("POST", "/__admin/invitations", `{"reviewers":["not a name!"]}`); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad username accepted: %d", resp.StatusCode)
	}

	// A candidate connects; their session carries the reviewers.
	c := api.New(ts.URL+Prefix, "0.1.0")
	da, _ := c.StartSignIn(context.Background(), api.DeviceRequest{Device: api.Device{OS: "linux", Arch: "amd64"}})
	if err := s.ApproveSignIn(da.UserCode, created.ID, ""); err != nil {
		t.Fatal(err)
	}
	res, err := c.PollSignIn(context.Background(), da.DeviceCode)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Session.GitHub.Reviewers; strings.Join(got, ",") != "alice,Bob" {
		t.Fatalf("session reviewers: %v", got)
	}

	// The hiring team changes; the running session sees it.
	if resp := admin("PUT", "/__admin/invitations/"+created.ID+"/reviewers", `{"reviewers":["alice","carol"]}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("put: %d", resp.StatusCode)
	}
	c.Token = res.SessionToken
	sess, _ := c.Session(context.Background())
	if got := sess.GitHub.Reviewers; strings.Join(got, ",") != "alice,carol" {
		t.Fatalf("after the change: %v", got)
	}
	resp = admin("GET", "/__admin/invitations", "")
	var list []struct {
		ID        string   `json:"invitation_id"`
		Reviewers []string `json:"reviewers"`
	}
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || strings.Join(list[0].Reviewers, ",") != "alice,carol" {
		t.Fatalf("list: %+v", list)
	}
	if resp := admin("PUT", "/__admin/invitations/inv_nope/reviewers", `{"reviewers":[]}`); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown invitation: %d", resp.StatusCode)
	}
}
