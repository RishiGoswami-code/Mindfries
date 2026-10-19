# `v2/db` — the `v2` schema

Migrations for V2's data, which lives in a `v2` schema in the existing
Supabase project. See [PLAN, Part 1 §3](../PLAN.md#3-its-own-data-beside-v1s).

**Status: empty.** Stage 2 adds the first migrations.

## What goes here

- Numbered SQL migrations that create and change objects in the `v2` schema
  only.
- Their own runner, tracking what has run in `v2.schema_migrations`, separate
  from V1's `schema_migrations`.
- The V2 Postgres role and its grants: full access to `v2`, read-only on the
  V1 tables V2 needs, nothing else.

## Rules

- Never `ALTER`, `DROP` or write to a V1 table from here. The one V1 change,
  `assessments.engine`, is a connection point and goes in
  `supabase/migrations/` in its own `v1-seam` PR.
- Tests use synthetic data only.
