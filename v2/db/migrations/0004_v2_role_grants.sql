-- The database-enforced isolation (v2/PLAN.md Part 1 §3): the V2 API connects
-- as its own Postgres role that can do everything in `v2` and only READ the
-- few V1 tables it needs — so a V2 bug cannot write a V1 table.
--
-- TEMPLATE — run when you create the V2 role, not with 0001–0003:
--   1. Replace <V2_PASSWORD> with a real secret (store it as the API's V2_ env).
--   2. Confirm the V1 table list matches what the engine actually reads.
-- Safe to re-run; grants are additive and idempotent.

-- 1. The role. Create it once; comment this out if it already exists.
do $$
begin
  if not exists (select 1 from pg_roles where rolname = 'v2_api') then
    create role v2_api login password '<V2_PASSWORD>';
  end if;
end$$;

-- 2. Full access to the v2 schema (current objects + anything added later).
grant usage on schema v2 to v2_api;
grant all privileges on all tables    in schema v2 to v2_api;
grant all privileges on all sequences in schema v2 to v2_api;
alter default privileges in schema v2 grant all on tables    to v2_api;
alter default privileges in schema v2 grant all on sequences to v2_api;

-- 3. Read-only on the V1 tables V2 keys against — SELECT only, never write.
grant usage on schema public to v2_api;
grant select on
  public.candidate_users,
  public.companies,
  public.company_users,
  public.job_roles,
  public.assessments
to v2_api;

-- 4. Nothing else. Do not grant any write on public.* to v2_api.
