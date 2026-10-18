-- Mindfries — companies can sign themselves up; credits gate their hiring.
--
-- A company that signs up on its own (company/frontend /signup) starts
-- with access 'pending' and 0 credits: it can sign in and look around, but
-- not create roles or invite candidates until a Mindfries admin grants
-- access and credits from internal-admin (/admin/access). Inviting a
-- candidate spends 1 credit.
--
-- Every company that existed before this, and any onboarded by Mindfries
-- the old way, keeps access 'active' and credits NULL — "not metered" —
-- so nothing changes for them.
--
-- Additive only. Apply after 0026_candidate_settings.sql.

alter table companies
  add column if not exists access_status text not null default 'active',  -- pending|active|declined
  add column if not exists credits int,                                     -- NULL: not metered; a number: the balance
  add column if not exists signup_source text not null default 'admin',    -- admin|self
  add column if not exists access_requested_at timestamptz;                 -- the company's latest "remind the team" nudge

do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'companies_access_status_check') then
    alter table companies add constraint companies_access_status_check
      check (access_status in ('pending', 'active', 'declined'));
  end if;
  if not exists (select 1 from pg_constraint where conname = 'companies_credits_check') then
    alter table companies add constraint companies_credits_check check (credits is null or credits >= 0);
  end if;
  if not exists (select 1 from pg_constraint where conname = 'companies_signup_source_check') then
    alter table companies add constraint companies_signup_source_check check (signup_source in ('admin', 'self'));
  end if;
end $$;

create index if not exists companies_access_idx on companies (access_status, signup_source);

-- Every grant, and who gave it: the record of why a company has the credits
-- it has.
create table if not exists company_credit_grants (
  id          uuid primary key default gen_random_uuid(),
  company_id  uuid not null references companies(id) on delete cascade,
  amount      int  not null check (amount > 0),
  plan        text,
  note        text,
  granted_by  text not null,                 -- the internal-admin account's email
  created_at  timestamptz not null default now()
);
create index if not exists company_credit_grants_company_idx on company_credit_grants (company_id, created_at desc);
alter table company_credit_grants enable row level security;

-- Spend one credit, atomically: the row is locked for the check and the
-- decrement, so two invites racing for a last credit can't both get it.
-- Returns the balance left, or NULL for a company that isn't metered.
-- Raises NO_ACCESS for a company without access, NO_CREDITS at 0.
create or replace function spend_company_credit(p_company uuid) returns int
language plpgsql as $$
declare
  v_status  text;
  v_credits int;
begin
  select access_status, credits into v_status, v_credits from companies where id = p_company for update;
  if not found then raise exception 'COMPANY_NOT_FOUND'; end if;
  if v_status <> 'active' then raise exception 'NO_ACCESS'; end if;
  if v_credits is null then return null; end if;
  if v_credits < 1 then raise exception 'NO_CREDITS'; end if;
  update companies set credits = credits - 1 where id = p_company;
  return v_credits - 1;
end $$;

-- Gives back a credit spent on an invitation that then couldn't be made.
create or replace function refund_company_credit(p_company uuid) returns void
language sql as $$
  update companies set credits = credits + 1 where id = p_company and credits is not null;
$$;

-- Grants access and adds credits in one step, and logs it. Returns the new
-- balance. Used by internal-admin only.
create or replace function grant_company_credits(p_company uuid, p_amount int, p_plan text, p_note text, p_by text) returns int
language plpgsql as $$
declare
  v_credits int;
begin
  if p_amount is null or p_amount < 1 then raise exception 'AMOUNT_MUST_BE_POSITIVE'; end if;
  update companies
     set access_status = 'active',
         credits = coalesce(credits, 0) + p_amount,
         plan = coalesce(nullif(p_plan, ''), plan)
   where id = p_company
  returning credits into v_credits;
  if not found then raise exception 'COMPANY_NOT_FOUND'; end if;
  insert into company_credit_grants (company_id, amount, plan, note, granted_by)
  values (p_company, p_amount, nullif(p_plan, ''), nullif(p_note, ''), p_by);
  return v_credits;
end $$;

-- Only the apps' service role calls these, never a browser key. (anon and
-- authenticated are Supabase's roles; skipped where they don't exist.)
revoke execute on function spend_company_credit(uuid) from public;
revoke execute on function refund_company_credit(uuid) from public;
revoke execute on function grant_company_credits(uuid, int, text, text, text) from public;
do $$
declare r text;
begin
  foreach r in array array['anon', 'authenticated'] loop
    if exists (select 1 from pg_roles where rolname = r) then
      execute format('revoke execute on function spend_company_credit(uuid) from %I', r);
      execute format('revoke execute on function refund_company_credit(uuid) from %I', r);
      execute format('revoke execute on function grant_company_credits(uuid, int, text, text, text) from %I', r);
    end if;
  end loop;
end $$;
