// Package store is the Postgres-backed persistence for the engine: the product
// registry (read-only) and the task cache, both in the `v2` schema. The server
// connects as the `v2_api` role (full `v2`, read-only on the V1 tables), so a
// bug here cannot write V1 data — the database enforces it.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mindfries/v2/api/internal/engine"
	"github.com/mindfries/v2/api/internal/manifest"
)

// DB is a pool of connections to the v2 schema.
type DB struct {
	pool *pgxpool.Pool
}

// Open connects using a Postgres URL (V2_DATABASE_URL) and verifies the
// connection.
func Open(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("store: parse url: %w", err)
	}
	// Use the simple protocol (no server-side prepared statements) so the pool
	// works behind Supabase's transaction pooler (port 6543) as well as a
	// direct/session connection. ponytail: drop this if you always connect
	// directly and want prepared-statement caching.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Close releases the pool.
func (d *DB) Close() { d.pool.Close() }

// Products returns the verified product registry, each with its parsed +
// validated role-coverage manifest. Implements httpapi.Registry.
func (d *DB) Products(ctx context.Context) ([]engine.Product, error) {
	rows, err := d.pool.Query(ctx,
		`select slug, name, domain, coalesce(golden_commit, ''), role_coverage
		   from v2.products
		  where status = 'verified'
		  order by slug`)
	if err != nil {
		return nil, fmt.Errorf("store: query products: %w", err)
	}
	defer rows.Close()

	var out []engine.Product
	for rows.Next() {
		var slug, name, domain, commit string
		var rc []byte
		if err := rows.Scan(&slug, &name, &domain, &commit, &rc); err != nil {
			return nil, err
		}
		m, err := manifest.Parse(rc)
		if err != nil {
			return nil, fmt.Errorf("store: product %q manifest: %w", slug, err)
		}
		out = append(out, engine.Product{
			Slug: slug, Name: name, Domain: domain, GoldenCommit: commit, Manifest: m,
		})
	}
	return out, rows.Err()
}
