-- A stand-in for the V1 tables v2_api reads, so the v2 migrations can be applied
-- to a throwaway database in CI. The real tables live in supabase/migrations.
create table if not exists public.candidate_users (id uuid primary key default gen_random_uuid());
create table if not exists public.companies       (id uuid primary key default gen_random_uuid());
create table if not exists public.company_users   (id uuid primary key default gen_random_uuid());
create table if not exists public.job_roles       (id uuid primary key default gen_random_uuid());
create table if not exists public.assessments     (id uuid primary key default gen_random_uuid());
