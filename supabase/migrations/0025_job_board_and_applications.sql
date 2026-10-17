-- Mindfries — the job board: companies publish roles, candidates apply, the
-- hiring team reviews them, schedules interviews and writes to them by email.
--
-- Additive only. Apply after 0024_candidate_job_preferences_and_documents.sql.
--
-- Assessments stay invite-only (0022): applying puts a candidate in front of
-- the hiring team, who invite the ones they want. A role can instead let
-- every applicant take its assessment (self_assessment) — the company's
-- choice, per role — in which case applying creates the invitation at once.

-- A role as a job posting. `listed` puts it on the candidates' job board
-- while the role is open; the rest is what the posting says.
alter table job_roles
  add column if not exists listed          boolean not null default false,
  add column if not exists self_assessment boolean not null default false,
  add column if not exists description     text,
  add column if not exists location        text,
  add column if not exists work_mode       text,      -- remote|hybrid|onsite
  add column if not exists employment_type text,      -- full_time|part_time|contract|internship
  add column if not exists published_at    timestamptz;

create index if not exists job_roles_listed_idx on job_roles (published_at desc) where listed and status = 'open';

do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'job_roles_work_mode_check') then
    alter table job_roles add constraint job_roles_work_mode_check
      check (work_mode is null or work_mode in ('remote', 'hybrid', 'onsite'));
  end if;
  if not exists (select 1 from pg_constraint where conname = 'job_roles_employment_type_check') then
    alter table job_roles add constraint job_roles_employment_type_check
      check (employment_type is null or employment_type in ('full_time', 'part_time', 'contract', 'internship'));
  end if;
end $$;

-- An application now has two ways in: a company's invitation ('invited',
-- as before) or the candidate applying from the job board ('applied').
-- Stages gain 'applied' (waiting for the hiring team), 'interviewing',
-- 'offered' and 'withdrawn' (the candidate pulled out) beside the existing
-- invited|in_progress|completed|shortlisted|rejected|hired|declined — still
-- text, by convention, as before.
alter table candidate_applications
  add column if not exists candidate_id     uuid references candidate_users(id) on delete set null,
  add column if not exists source           text not null default 'invited',   -- invited|applied
  add column if not exists cover_note       text,
  -- When the stage last changed, so the candidate's timeline can date it.
  add column if not exists stage_changed_at timestamptz;

create index if not exists candidate_applications_candidate_idx on candidate_applications (candidate_id);
create index if not exists candidate_applications_email_idx on candidate_applications (lower(candidate_email));

-- Interviews the hiring team schedules for an application. The candidate
-- can confirm one, ask for another time, or cancel it, with a message.
create table if not exists application_interviews (
  id               uuid primary key default gen_random_uuid(),
  application_id   uuid not null references candidate_applications(id) on delete cascade,
  company_id       uuid not null references companies(id) on delete cascade,
  title            text not null,                  -- "Technical interview", "Recruiter screen"…
  starts_at        timestamptz not null,
  duration_min     int not null default 45,
  -- A meeting link or an address. Shown to the candidate as written.
  location         text,
  notes            text,
  status           text not null default 'scheduled',
  -- What the candidate said with a confirm, a reschedule request or a cancellation.
  candidate_message text,
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now(),
  constraint application_interviews_status_check
    check (status in ('scheduled', 'confirmed', 'reschedule_requested', 'cancelled_by_candidate', 'cancelled', 'completed'))
);
create index if not exists application_interviews_application_idx on application_interviews (application_id, starts_at);
create index if not exists application_interviews_company_idx on application_interviews (company_id, starts_at);

-- Email the hiring team sent a candidate from the Company Portal — the
-- only messaging there is. Kept so both sides can see what was sent; the
-- candidate replies from their own mailbox, to the sender's address.
create table if not exists application_emails (
  id             uuid primary key default gen_random_uuid(),
  application_id uuid not null references candidate_applications(id) on delete cascade,
  company_id     uuid not null references companies(id) on delete cascade,
  sent_by        uuid references company_users(id) on delete set null,
  sent_by_name   text,
  subject        text not null,
  body           text not null,
  -- False when the mail service refused it; the error is kept to show the sender.
  delivered      boolean not null default false,
  error          text,
  sent_at        timestamptz not null default now()
);
create index if not exists application_emails_application_idx on application_emails (application_id, sent_at desc);

alter table application_interviews enable row level security;
alter table application_emails enable row level security;
