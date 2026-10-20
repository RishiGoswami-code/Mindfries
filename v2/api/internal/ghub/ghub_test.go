package ghub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRepo(t *testing.T) {
	ok, err := ParseRepo("ses_1", "octocandidate", "octocandidate/mindfries-x", "https://github.com/octocandidate/mindfries-x", "main", true, []string{"rev"})
	if err != nil || ok.Owner != "octocandidate" || ok.Name != "mindfries-x" || !ok.Private {
		t.Fatalf("%+v %v", ok, err)
	}
	bad := []struct {
		login, full, url, branch string
		private                  bool
	}{
		{"a", "no-slash", "https://x.test/a", "main", true},
		{"a", "a/b/c", "https://x.test/a", "main", true},
		{"someone-else", "a/b", "https://x.test/a", "main", true},
		{"a", "a/b", "http://x.test/a", "main", true},
		{"a", "a/b", "https://x.test/a", "main", false},
		{"a", "a/b", "https://x.test/a", "", true},
		{"a", "a/..", "https://x.test/a", "main", true},
	}
	for i, b := range bad {
		if _, err := ParseRepo("s", b.login, b.full, b.url, b.branch, b.private, nil); err == nil {
			t.Errorf("case %d should be refused", i)
		}
	}
}

func TestClientAgainstAFakeGitHub(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.PostForm.Get("code") == "good" && r.PostForm.Get("client_secret") == "shh" {
			_, _ = w.Write([]byte(`{"access_token":"gho_abc","scope":"repo","token_type":"bearer"}`))
			return
		}
		_, _ = w.Write([]byte(`{"error":"bad_verification_code"}`))
	})
	mux.HandleFunc("GET /api/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gho_abc" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"login":"octocandidate","id":42}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := Client{WebURL: ts.URL, APIURL: ts.URL + "/api", ClientID: "cid", ClientSecret: "shh"}

	u := c.AuthorizeURL("https://api.test/github/callback", "st1")
	if !strings.HasPrefix(u, ts.URL+"/login/oauth/authorize?") || !strings.Contains(u, "client_id=cid") ||
		!strings.Contains(u, "scope=repo") || !strings.Contains(u, "state=st1") {
		t.Fatal(u)
	}
	tok, scope, err := c.Exchange(context.Background(), "good", "x")
	if err != nil || tok != "gho_abc" || scope != "repo" {
		t.Fatalf("%q %q %v", tok, scope, err)
	}
	if _, _, err := c.Exchange(context.Background(), "bad", "x"); err == nil || !strings.Contains(err.Error(), "bad verification code") {
		t.Fatalf("a bad code: %v", err)
	}
	login, id, err := c.User(context.Background(), "gho_abc")
	if err != nil || login != "octocandidate" || id != 42 {
		t.Fatalf("%q %d %v", login, id, err)
	}
	if _, _, err := c.User(context.Background(), "nope"); err == nil {
		t.Fatal("a refused token is an error")
	}
	if (Client{}).web() != "https://github.com" || (Client{}).api() != "https://api.github.com" {
		t.Fatal("defaults are github.com")
	}
}
