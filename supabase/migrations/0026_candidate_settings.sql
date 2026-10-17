-- Mindfries — a candidate's account settings: which emails they want, and
-- what of their profile hiring teams may see.
--
-- Additive only. Apply after 0025_job_board_and_applications.sql.

-- Emails a candidate can turn off. Shape (written and checked by
-- candidate/frontend/src/lib/settings/shape.ts):
--   { invitations: bool, interviews: bool, messages: bool }
-- A missing key means on. Off means the company portal doesn't email that
-- kind — it still records it, and the candidate still sees it in the
-- portal, and the company is told why no email went. Sign-in, password and
-- address emails are not in here: they always go.
alter table candidate_users
  add column if not exists email_prefs jsonb not null default '{}'::jsonb;

-- Parts of the profile the candidate has hidden from hiring teams. The
-- company portal (company/frontend/lib/candidate-profile.ts) leaves these
-- out of what it shows. Name, email and the evidence from an assessment the
-- company itself sent are not hideable.
alter table candidate_users
  add column if not exists hidden_fields text[] not null default '{}';

do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'candidate_users_hidden_fields_check') then
    alter table candidate_users add constraint candidate_users_hidden_fields_check
      check (hidden_fields <@ array['summary', 'location', 'photo', 'code', 'linkedin', 'portfolio', 'resume', 'preferences', 'documents']::text[]);
  end if;
end $$;

-- Projects from a linked GitHub or GitLab account that the candidate chose
-- not to show, by the project's address. Everything else read from the
-- account shows.
alter table candidate_users
  add column if not exists hidden_projects text[] not null default '{}';
