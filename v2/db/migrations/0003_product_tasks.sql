-- The task bank + cache. A task is a role-scoped work item ON a product (a
-- feature to add, a planted bug to fix, a review/design prompt), not a
-- codebase. Generated once per (product, commit, role, JD) and reused across
-- every candidate on that JD; a company editing the JD changes jd_hash and so
-- regenerates. See v2/codebase-engine/ARCHITECTURE.md part 3.
--
-- The heavy content (starter/solution/test files) is NOT stored here — rows
-- reference the product + commit + an overlay, so a task row stays small.
-- jd_hash (not a FK to V1 job_roles) is the cache key, which keeps V2
-- decoupled from V1's schema.

create table if not exists v2.product_tasks (
  id             uuid primary key default gen_random_uuid(),
  product_id     uuid not null references v2.products(id) on delete cascade,
  golden_commit  text not null,                        -- the commit this task is cut from

  -- cache key: one pool of variants per (product, commit, role, jd_hash)
  role           text not null
                 check (role in ('backend', 'frontend', 'fullstack', 'fde',
                                 'quant', 'data_analyst', 'ml_researcher', 'ai_engineer')),
  jd_hash        text not null,                         -- hash(normalized JD text)
  variant_no     int  not null default 0,               -- which variant in the pool

  task_version   text not null,                          -- stable id a running session pins to
  kind           text not null
                 check (kind in ('feature', 'debug', 'review', 'design')),

  brief_md       text not null default '',               -- candidate-facing TASK.md
  overlay_ref    text,                                   -- task overlay files (incl. planted-bug diff for 'debug')
  acceptance_ref text,                                   -- hidden acceptance tests (never shipped to the candidate)
  solution_ref   text,                                   -- reference solution branch (verification + rubric calibration)
  rubric_ref     text,                                   -- the role rubric this task is calibrated against
  verification   jsonb not null default '{}'::jsonb,     -- last verify run: pass/fail, which tests

  created_at     timestamptz not null default now(),

  unique (product_id, golden_commit, role, jd_hash, variant_no)
);

-- Lookups on invite are by the cache key without the variant: fetch the pool,
-- draw a variant.
create index if not exists product_tasks_pool_idx
  on v2.product_tasks (product_id, role, jd_hash);

insert into v2.schema_migrations (version) values ('0003_product_tasks')
  on conflict (version) do nothing;
