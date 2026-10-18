package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnconfiguredSendsNothing(t *testing.T) {
	m := New("", "from@x", "to@y", "") // no key
	if m.Configured() {
		t.Fatal("a Mailer missing its key must not report Configured")
	}
	if err := m.TaskFailure(context.Background(), "inv1", "Acme", "Backend", "a@b.com", "boom"); err == nil {
		t.Fatal("an unconfigured Mailer must error rather than silently claim it sent")
	}
}

func TestTaskFailurePostsTheAlert(t *testing.T) {
	var gotPath, gotAuth string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := New("key123", "from@x", "to@y", srv.URL)
	if !m.Configured() {
		t.Fatal("expected Configured with key, from and to all set")
	}
	if err := m.TaskFailure(context.Background(), "inv1", "Acme", "Backend engineer", "cand@x.com", "too long"); err != nil {
		t.Fatalf("TaskFailure: %v", err)
	}
	if gotPath != "/emails" {
		t.Errorf("path = %q, want /emails", gotPath)
	}
	if gotAuth != "Bearer key123" {
		t.Errorf("auth = %q, want Bearer key123", gotAuth)
	}
	if body["to"] != "to@y" {
		t.Errorf("to = %v, want to@y", body["to"])
	}
	// The alert has to name who is blocked, or it's useless to whoever reads it.
	if s, _ := body["text"].(string); !strings.Contains(s, "cand@x.com") {
		t.Errorf("alert body should name the candidate; got %q", s)
	}
}

func TestSendSurfacesProviderRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()
	m := New("key", "from@x", "to@y", srv.URL)
	if err := m.TaskFailure(context.Background(), "inv1", "Acme", "Backend", "a@b.com", "boom"); err == nil {
		t.Fatal("a non-2xx from the provider must be returned as an error, not swallowed")
	}
}
