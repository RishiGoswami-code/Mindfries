package blob

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMem(t *testing.T) {
	m := NewMem()
	ctx := context.Background()
	if _, err := m.SignedURL(ctx, "a/b", time.Minute); err != ErrNotFound {
		t.Fatal(err)
	}
	if err := m.Put(ctx, "a/b", "image/jpeg", []byte("x")); err != nil || m.Count() != 1 {
		t.Fatal(err)
	}
	if u, err := m.SignedURL(ctx, "a/b", time.Minute); err != nil || u == "" {
		t.Fatal(err)
	}
	_ = m.Delete(ctx, []string{"a/b", "never-existed"})
	if m.Count() != 0 {
		t.Fatal("deleted")
	}
}

func TestSupabase(t *testing.T) {
	var gotAuth, gotPath, gotType, gotUpsert string
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /storage/v1/object/v2-evidence/", func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotType, gotUpsert = r.Header.Get("Authorization"), r.URL.EscapedPath(), r.Header.Get("Content-Type"), r.Header.Get("x-upsert")
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(200)
	})
	mux.HandleFunc("POST /storage/v1/object/sign/v2-evidence/", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ ExpiresIn int }
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.ExpiresIn != 300 {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write([]byte(`{"signedURL":"/object/sign/v2-evidence/sessions/s1/still.jpg?token=abc"}`))
	})
	mux.HandleFunc("DELETE /storage/v1/object/v2-evidence", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Prefixes []string }
		_ = json.NewDecoder(r.Body).Decode(&b)
		deleted = b.Prefixes
		w.WriteHeader(200)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	s := Supabase{BaseURL: ts.URL, Key: "service-key", Bucket: "v2-evidence"}
	ctx := context.Background()

	if err := s.Put(ctx, "sessions/s1/camera still.jpg", "image/jpeg", []byte("jpg")); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer service-key" || gotPath != "/storage/v1/object/v2-evidence/sessions/s1/camera%20still.jpg" ||
		gotType != "image/jpeg" || gotUpsert != "true" {
		t.Fatalf("%q %q %q %q", gotAuth, gotPath, gotType, gotUpsert)
	}
	u, err := s.SignedURL(ctx, "sessions/s1/still.jpg", 5*time.Minute)
	if err != nil || u != ts.URL+"/storage/v1/object/sign/v2-evidence/sessions/s1/still.jpg?token=abc" {
		t.Fatalf("%q %v", u, err)
	}
	if err := s.Delete(ctx, []string{"a", "b"}); err != nil || len(deleted) != 2 {
		t.Fatalf("%v %v", deleted, err)
	}
	bad := Supabase{BaseURL: ts.URL, Key: "k", Bucket: "missing"}
	if err := bad.Put(ctx, "x", "text/plain", nil); err == nil {
		t.Fatal("an unknown bucket is an error")
	}
}
