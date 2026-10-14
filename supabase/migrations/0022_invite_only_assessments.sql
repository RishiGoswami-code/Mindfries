-- Mindfries — assessments by invitation only, each with its own link and its
-- own generated task.
--
-- Apply after 0021_assessments_candidate_id.sql.
--
-- A candidate can only take an assessment they were invited to — by a
-- company from the Company Portal, or by Mindfries from the admin portal.
-- Each invitation carries:
--   * a private link (invite_token) that opens that one assessment, for the
--     invited email only — the candidate still signs in as that address;
--   * its own task, generated for that candidate from the role's brief, so no
--     two candidates for a role get the same code. candidate/backend
--     generates it after the invitation is created (task_status: pending →
--     generating → ready, or failed with task_error).

-- What a role assesses. The brief is what each candidate's task is
-- generated from: the work, the level, what to look for.
alter table job_roles
  add column if not exists assessment_brief text,
  add column if not exists task_variant     text not null default 'bug_fix';

alter table assessments
  -- The link. Made by the database for every invitation, so neither portal
  -- can forget it; two UUIDs' worth of randomness, as hex.
  add column if not exists invite_token text
    default replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', ''),
  -- The role the invitation is for, so the backend knows which brief to
  -- generate from. Null for an invitation from before roles existed.
  add column if not exists job_role_id uuid references job_roles(id) on delete set null,
  -- 'ready' for every invitation that already has its task (all of them,
  -- before this migration); new ones start 'pending'.
  add column if not exists task_status text not null default 'ready',
  add column if not exists task_error text,
  add column if not exists task_attempts int not null default 0,
  add column if not exists task_started_at timestamptz,
  -- Who sent it: 'company' or 'mindfries'.
  add column if not exists invited_by text;

-- Invitations made before this migration get a link too.
update assessments
  set invite_token = replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', '')
  where invite_token is null;

create unique index if not exists assessments_invite_token_key on assessments (invite_token);
create index if not exists assessments_task_pending_idx on assessments (task_status) where task_status in ('pending', 'generating');

do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'assessments_task_status_check') then
    alter table assessments add constraint assessments_task_status_check
      check (task_status in ('pending', 'generating', 'ready', 'failed'));
  end if;
end $$;
