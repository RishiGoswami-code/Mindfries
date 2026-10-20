-- V2 lives in its own `v2` schema beside V1's `public`, so a V2 change can
-- never touch a V1 table (v2/PLAN.md Part 1 §3). This is the first V2
-- migration: it creates the schema and the V2-only migration ledger.
--
-- Run order: 0001 (this) -> 0002_products -> 0003_product_tasks.
-- Idempotent: safe to re-run; `if not exists` throughout.

create schema if not exists v2;

-- V2 tracks its own applied migrations, separate from V1's schema_migrations.
-- Each later file records itself at the end.
create table if not exists v2.schema_migrations (
  version    text primary key,
  applied_at timestamptz not null default now()
);

insert into v2.schema_migrations (version) values ('0001_v2_schema')
  on conflict (version) do nothing;
