-- The candidate's notice and consent, who read sensitive evidence, and the
-- candidate's own words about a flag. Run after 0017.

-- What the candidate was told is collected, and that they accepted it, per
-- assessment. The version says which wording they saw.
create table if not exists v2.consents (
  candidate_id  uuid not null,
  assessment_id text not null,
  version       text not null,
  items         jsonb not null default '[]'::jsonb,   -- what was listed: camera, screen, microphone, activity...
  accepted_at   timestamptz not null default now(),
  primary key (candidate_id, assessment_id, version)
);
create index if not exists consents_assessment_idx on v2.consents (assessment_id);

-- Every read of sensitive evidence by staff: who, what, when.
create table if not exists v2.evidence_access (
  id          bigserial primary key,
  session_id  text not null,                           -- kept even if the session is deleted
  actor       text not null,                           -- "company:<user>" or "admin:<name>", as the portal says
  action      text not null,                           -- evidence | media | report | transcript
  detail      text not null default '',                -- e.g. the evidence id
  at          timestamptz not null default now()
);
create index if not exists evidence_access_session_idx on v2.evidence_access (session_id, at);

-- The candidate's explanation of a flag or notice, in their own words.
create table if not exists v2.flag_responses (
  activity_id bigint primary key references v2.activity(id) on delete cascade,
  text        text not null,
  at          timestamptz not null default now()
);

alter table v2.consents         disable row level security;
alter table v2.evidence_access  disable row level security;
alter table v2.flag_responses   disable row level security;

insert into v2.schema_migrations (version) values ('0026_consent_audit_responses')
  on conflict (version) do nothing;
