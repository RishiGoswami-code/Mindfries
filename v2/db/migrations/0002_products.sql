-- The product registry: the few large, owned, test-green codebases that many
-- roles are assessed on (v2/codebase-engine/ARCHITECTURE.md part 1, ANALYSIS.md
-- for the 5-product set). A product is authored once and pinned at a verified
-- `golden_commit`; tasks are cut from it rather than codebases being generated
-- per candidate.
--
-- Not keyed to any V1 table: products are a global catalogue, not company- or
-- candidate-specific, so there is no cross-schema coupling to V1.

create table if not exists v2.products (
  id            uuid primary key default gen_random_uuid(),
  slug          text not null unique,                 -- 'p1-support-saas'
  name          text not null,                        -- 'Support SaaS'
  domain        text not null,                         -- 'customer-support SaaS'
  repo_ref      text,                                  -- where the source lives (V2-owned git / object store)
  golden_commit text,                                  -- pinned, verified (suite green) commit tasks are cut from
  stack         jsonb not null default '[]'::jsonb,    -- ["nextjs","go","postgres"] — feeds CLI preflight runtimes
  -- role -> { subsystems:[paths], competencies:[...], modes:[...], cannot_assess:[...] }
  -- the manifest that drives selection and honest "not assessed" reporting.
  role_coverage jsonb not null default '{}'::jsonb,
  status        text not null default 'draft'
                check (status in ('draft', 'verified', 'retired')),
  version       int  not null default 1,               -- bumped when golden_commit changes
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now()
);

insert into v2.schema_migrations (version) values ('0002_products')
  on conflict (version) do nothing;
