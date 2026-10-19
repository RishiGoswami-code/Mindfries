package fakeapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminToken(t *testing.T) {
	s := New(Config{AdminToken: "s3cret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	post := func(token string) int {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/__admin/setup-codes", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if c := post(""); c != http.StatusUnauthorized {
		t.Fatalf("no token: %d", c)
	}
	if c := post("wrong"); c != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", c)
	}
	if c := post("s3cret"); c != http.StatusCreated {
		t.Fatalf("right token: %d", c)
	}
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz: %v %v", resp, err)
	}
	// The contract itself needs no admin token.
	resp, _ = http.Get(ts.URL + Prefix + "/version")
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(resp.Header.Get("Content-Type"), "problem") {
		t.Fatalf("contract route: %d", resp.StatusCode)
	}
}
