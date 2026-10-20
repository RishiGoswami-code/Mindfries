# `v2/db` — the `v2` schema

Migrations for V2's data, which lives in a `v2` schema in the existing
Supabase project. See [PLAN, Part 1 §3](../PLAN.md#3-its-own-data-beside-v1s).

**Status: first migrations landed** — the codebase-engine registry
(`migrations/`). The rest of Stage 2 (sessions, setup codes, events,
submissions) still to come.

## Migrations (run in order)

In `migrations/`. Idempotent (safe to re-run). Run them in the Supabase SQL
editor top to bottom:

| File | Creates |
|---|---|
| `0001_v2_schema.sql` | the `v2` schema and `v2.schema_migrations` ledger |
| `0002_products.sql` | `v2.products` — the owned product registry |
| `0003_product_tasks.sql` | `v2.product_tasks` — the task bank + JD cache |
| `0004_v2_role_grants.sql` | **template** — the `v2_api` role and its grants; run only when you create the role (fill in the password first) |

Design behind these: [`../codebase-engine/ARCHITECTURE.md`](../codebase-engine/ARCHITECTURE.md).

## What goes here

- Numbered SQL migrations that create and change objects in the `v2` schema
  only.
- Their own ledger in `v2.schema_migrations`, separate from V1's
  `schema_migrations`. (Each file records itself; no runner yet — migrations
  are applied by hand in the SQL editor for now.)
- The V2 Postgres role and its grants: full access to `v2`, read-only on the
  V1 tables V2 needs, nothing else (`0004`).

## Rules

- Never `ALTER`, `DROP` or write to a V1 table from here. The one V1 change,
  `assessments.engine`, is a connection point and goes in
  `supabase/migrations/` in its own `v1-seam` PR.
- Tests use synthetic data only.
