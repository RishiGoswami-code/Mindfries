package engine

import "testing"

func TestNormalizeJDIgnoresFormatting(t *testing.T) {
	a := NormalizeJD("Senior  Backend Engineer!!  (Go, Postgres)\n")
	b := NormalizeJD("senior backend engineer go postgres")
	if a != b {
		t.Fatalf("formatting should not matter:\n %q\n %q", a, b)
	}
}

func TestHashJDStableAndSensitive(t *testing.T) {
	// Same content, different formatting -> same hash (cache reuse).
	if HashJD("Backend Engineer - Go") != HashJD("backend   engineer go") {
		t.Fatal("cosmetic edits must not change the cache key")
	}
	// Real content change -> different hash (triggers regeneration).
	if HashJD("Backend Engineer - Go") == HashJD("Backend Engineer - Rust") {
		t.Fatal("a content change must change the cache key")
	}
}
