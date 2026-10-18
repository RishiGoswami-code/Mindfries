package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func loadEnvFile(p string) {
	f, err := os.Open(p)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

func main() {
	loadEnvFile(".env.local")
	loadEnvFile("../frontend/.env.local")
	loadEnvFile("../../supabase/.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Connecting to database: %v", err)
	}
	defer conn.Close(ctx)

	fmt.Println("Connected to database.")

	_, err = conn.Exec(ctx, `
		create table if not exists schema_migrations (
			filename    text primary key,
			applied_at  timestamptz not null default now()
		);
	`)
	if err != nil {
		log.Fatalf("Creating schema_migrations: %v", err)
	}

	rows, err := conn.Query(ctx, "select filename from schema_migrations")
	if err != nil {
		log.Fatalf("Querying schema_migrations: %v", err)
	}
	defer rows.Close()

	done := make(map[string]bool)
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err == nil {
			done[fn] = true
		}
	}

	migrationsDir := "../../supabase/migrations"
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatalf("Reading migrations dir %s: %v", migrationsDir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var pending []string
	for _, f := range files {
		if done[f] {
			fmt.Printf("  = %s (already applied)\n", f)
		} else {
			pending = append(pending, f)
		}
	}

	if len(pending) == 0 {
		fmt.Println("\nNothing to do — database is up to date.")
		return
	}

	fmt.Printf("\nApplying %d pending migration(s):\n", len(pending))
	for _, f := range pending {
		fmt.Printf("  + %s ... ", f)
		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			log.Fatalf("Reading %s: %v", f, err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			log.Fatalf("Begin tx: %v", err)
		}

		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			tx.Rollback(ctx)
			log.Fatalf("Failed executing %s: %v", f, err)
		}

		if _, err := tx.Exec(ctx, "insert into schema_migrations (filename) values ($1)", f); err != nil {
			tx.Rollback(ctx)
			log.Fatalf("Recording %s in schema_migrations: %v", f, err)
		}

		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("Commit %s: %v", f, err)
		}
		fmt.Println("OK")
	}

	fmt.Println("\nAll pending migrations applied successfully!")
}
