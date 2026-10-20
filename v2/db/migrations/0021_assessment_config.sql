-- B1.5: per-assessment configuration. The reviewers invited to the candidate's
-- repository, the time budget, the AI policy and what monitoring is required
-- are set per assessment (never in an environment variable). Keyed by the V1
-- assessment id (uuid, soft link). Run after 0017.

create table if not exists v2.assessment_config (
  assessment_id     uuid primary key,
  duration_minutes  int  not null default 90 check (duration_minutes between 10 and 600),
  assistant_mode    text not null default 'explain_only' check (assistant_mode in ('off', 'explain_only', 'code_allowed')),
  outside_agents_allowed boolean not null default false,
  reviewers         jsonb not null default '[]'::jsonb,       -- GitHub logins
  require_camera    boolean not null default true,
  require_screen    boolean not null default true,
  require_audio     boolean not null default true,
  avatar_enabled    boolean not null default true,
  tabs_override     jsonb,                                    -- null: derive from the manifest's modes
  jd                text not null default '',
  updated_at        timestamptz not null default now(),
  updated_by        text not null default ''
);

alter table v2.assessment_config disable row level security;

insert into v2.schema_migrations (version) values ('0021_assessment_config')
  on conflict (version) do nothing;
