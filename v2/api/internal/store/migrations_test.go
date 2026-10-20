package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestMigrationsApplyTwice applies every v2/db migration to a throwaway
// database, then again, and checks the second pass changes nothing and fails
// nothing (v2/PLAN.md task 2.1). Runs only with V2_TEST_DATABASE_URL, a
// superuser URL for a database that may be written to freely (CI's Postgres
// service container). It also proves v2_api cannot write a V1 table.
func TestMigrationsApplyTwice(t *testing.T) {
	url := os.Getenv("V2_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set V2_TEST_DATABASE_URL to a disposable Postgres to run the migration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	dir := filepath.Join("..", "..", "..", "db")
	apply := func(path string) {
		t.Helper()
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(path), err)
		}
	}
	apply(filepath.Join(dir, "testdata", "v1_stub.sql"))

	files, _ := filepath.Glob(filepath.Join(dir, "migrations", "*.sql"))
	sort.Strings(files)
	if len(files) < 13 {
		t.Fatalf("found only %d migrations", len(files))
	}
	for pass := 1; pass <= 2; pass++ {
		for _, f := range files {
			apply(f)
		}
	}

	var n int
	if err := conn.QueryRow(ctx, `select count(*) from v2.schema_migrations`).Scan(&n); err != nil || n < 13 {
		t.Fatalf("ledger has %d entries (%v)", n, err)
	}

	// The tables the server needs all exist.
	for _, tbl := range []string{"sessions", "setup_codes", "events", "snapshots", "submissions", "device_codes",
		"github_connections", "github_oauth_states", "session_repos", "monitoring_state", "evidence_objects",
		"monitoring_gaps", "assessment_config", "activity", "flag_reviews", "artifacts", "artifact_versions",
		"check_runs", "assistant_turns", "assistant_steps", "model_usage", "deletion_log"} {
		var ok bool
		if err := conn.QueryRow(ctx, `select to_regclass('v2.`+tbl+`') is not null`).Scan(&ok); err != nil || !ok {
			t.Errorf("v2.%s is missing", tbl)
		}
	}

	_, _ = conn.Exec(ctx, `delete from v2.sessions where id in ('ses_t1', 'ses_t2')`)

	// 'ready' is a legal session status; an unknown one is not.
	if _, err := conn.Exec(ctx, `insert into v2.sessions (id, token_hash, role, product_slug, bank_task_id, task_version, ends_at, status)
		values ('ses_t1', 'h1', 'backend', 'p1', 't', 'v1', now(), 'ready')`); err != nil {
		t.Fatalf("ready session: %v", err)
	}
	if _, err := conn.Exec(ctx, `insert into v2.sessions (id, token_hash, role, product_slug, bank_task_id, task_version, ends_at, status)
		values ('ses_t2', 'h2', 'backend', 'p1', 't', 'v1', now(), 'nonsense')`); err == nil {
		t.Fatal("an unknown status must be refused")
	}

	// The V2 role can write v2 and may not write V1.
	if _, err := conn.Exec(ctx, `set role v2_api`); err != nil {
		t.Fatalf("set role: %v", err)
	}
	defer conn.Exec(ctx, `reset role`)
	if _, err := conn.Exec(ctx, `insert into v2.activity (session_id, severity, source, text) values ('ses_t1', 'info', 'server', 'ok')`); err != nil {
		t.Fatalf("v2_api must write v2: %v", err)
	}
	for _, q := range []string{
		`insert into public.assessments default values`,
		`delete from public.candidate_users`,
		`update public.companies set id = id`,
	} {
		if _, err := conn.Exec(ctx, q); err == nil {
			t.Fatalf("v2_api must not run %q", q)
		}
	}
}
