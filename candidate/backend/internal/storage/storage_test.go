package storage

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestFetchLive pulls a real still from the proctor-snapshots bucket. Skipped
// unless SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY and MF_TEST_STILL (an object
// path like "<session>/screen-....jpg") are set — a manual check, never CI.
func TestFetchLive(t *testing.T) {
	url := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	key := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	path := os.Getenv("MF_TEST_STILL")
	if url == "" || key == "" || path == "" {
		t.Skip("set SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY and MF_TEST_STILL to run the live fetch check")
	}
	data, ctype, err := New(url, key).Fetch(context.Background(), "proctor-snapshots", path)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("fetched an empty object")
	}
	t.Logf("fetched %d bytes, content-type %q", len(data), ctype)
}

func TestNewNilWhenUnconfigured(t *testing.T) {
	if New("", "") != nil {
		t.Fatal("expected nil client with no URL/key")
	}
	if New("https://x.supabase.co", "").Configured() {
		t.Fatal("expected not configured with no key")
	}
}
