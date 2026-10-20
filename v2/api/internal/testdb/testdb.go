// Package testdb gives tests a migrated Postgres. It is for tests only: it
// reads V2_TEST_DATABASE_URL (a superuser URL for a database that may be
// written to freely: CI's Postgres service, or a local one) and applies the
// V1 stand-in plus every v2/db migration once per test process. Without the
// variable, tests that need a database skip.
package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	once    sync.Once
	initErr error
)

// URL returns the test database's URL with the migrations applied, or skips
// the test when no database is configured.
func URL(t testing.TB) string {
	t.Helper()
	url := os.Getenv("V2_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set V2_TEST_DATABASE_URL to a disposable Postgres to run database tests")
	}
	once.Do(func() { initErr = migrate(url) })
	if initErr != nil {
		t.Fatalf("testdb: %v", initErr)
	}
	return url
}

func migrate(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	// Packages' tests run in parallel processes against one database; only one
	// at a time may migrate it.
	if _, err := conn.Exec(ctx, `select pg_advisory_lock(727200727)`); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), `select pg_advisory_unlock(727200727)`)
	if os.Getenv("V2_TEST_RESET") == "1" { // a clean slate for a local database whose schema drifted (run with -p 1)
		if _, err := conn.Exec(ctx, `drop schema if exists v2 cascade;
			drop table if exists public.candidate_users, public.companies, public.company_users, public.job_roles, public.assessments cascade;`); err != nil {
			return err
		}
	}
	dir := dbDir()
	files := []string{filepath.Join(dir, "testdata", "v1_stub.sql")}
	ms, _ := filepath.Glob(filepath.Join(dir, "migrations", "*.sql"))
	sort.Strings(ms)
	files = append(files, ms...)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

// dbDir is v2/db, found from this file's location.
func dbDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db")
}
