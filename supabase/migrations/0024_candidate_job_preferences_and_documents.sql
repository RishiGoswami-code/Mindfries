-- Mindfries — two things a candidate can now add to their profile: the work
-- they're looking for, and documents that back up what they say.
--
-- Additive only. Apply after 0023_admin_login_throttle.sql.

-- What the candidate wants next: the roles they're after, the employment
-- types they'd take, where, and the salary range they expect. Shape (written
-- and checked by candidate/frontend/src/lib/profile/job-preferences.ts):
--   { roles: text[], employment: text[], locations: text[],
--     salary: { min: int, max: int, currency: text } | null, updatedAt: text }
-- Work mode is not in here: it's candidate_users.open_to, the same field the
-- profile's edit form already sets, so there is one answer, not two.
-- Null means the candidate hasn't set any.
alter table candidate_users
  add column if not exists job_preferences jsonb;

-- Supporting documents — certificates, project reports and the like. The
-- file is in the private `candidate-documents` bucket under the candidate's
-- id; this row is what the profile and the hiring team's view list.
create table if not exists candidate_documents (
  id           uuid primary key default gen_random_uuid(),
  candidate_id uuid not null references candidate_users(id) on delete cascade,
  kind         text not null check (kind in ('certificate', 'report', 'other')),
  title        text not null,
  path         text not null,
  mime_type    text not null,
  size_bytes   integer not null,
  uploaded_at  timestamptz not null default now()
);
create index if not exists candidate_documents_candidate_idx on candidate_documents (candidate_id, uploaded_at desc);

alter table candidate_documents enable row level security;

insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values (
  'candidate-documents',
  'candidate-documents',
  false,
  10485760, -- 10MB
  array['application/pdf', 'image/png', 'image/jpeg']
)
on conflict (id) do update set
  file_size_limit = excluded.file_size_limit,
  allowed_mime_types = excluded.allowed_mime_types;
