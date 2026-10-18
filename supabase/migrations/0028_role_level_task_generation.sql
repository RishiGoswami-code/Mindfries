-- Task generation moves from per-candidate to per-role.
--
-- Until now each invitation generated its own task (assessments.task_status,
-- migration 0022): one full model generation per candidate, and the candidate
-- watched a "preparing" screen while it ran. A company describes a role once,
-- though, so the task can be made once for the role and shared by everyone
-- invited to it — generated when the role is created or its brief is edited,
-- so it is ready before any invitation goes out.
--
-- job_roles already carries template_id (the shared task) and assessment_brief
-- /task_variant (what to generate from). This adds the same task-generation
-- bookkeeping the assessments table got in 0022, so the backend worker can
-- claim a role, generate its task, and link it.

alter table job_roles
  -- Mirrors assessments.task_status, with one extra state: 'none', for a role
  -- that isn't having a task generated at the role level (every role that
  -- exists before this migration — they keep whatever they did before, either
  -- a template already chosen or the old per-invitation generation).
  add column if not exists task_status    text not null default 'pending',
  add column if not exists task_error     text,
  add column if not exists task_attempts  int  not null default 0,
  add column if not exists task_started_at timestamptz;

-- Existing roles are left as they are: a role that already has a task is
-- 'ready'; one that doesn't is 'none' (so the worker doesn't suddenly generate
-- a task for every role ever created — those invitations still generate their
-- own until the company next saves the role). New roles default to 'pending'
-- and are picked up by the worker.
update job_roles
  set task_status = case when template_id is not null then 'ready' else 'none' end
  where task_status = 'pending';

create index if not exists job_roles_task_pending_idx
  on job_roles (task_status) where task_status in ('pending', 'generating');

do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'job_roles_task_status_check') then
    alter table job_roles add constraint job_roles_task_status_check
      check (task_status in ('none', 'pending', 'generating', 'ready', 'failed'));
  end if;
end $$;
