-- V2 assessment sessions: the setup codes the dashboard mints, and the sessions
-- the CLI exchanges them for. Internal to the v2_api server (like products),
-- so RLS is disabled (see 0008). Run after 0001-0003.

create table if not exists v2.setup_codes (
  code             text primary key,
  assessment_title text not null default '',
  role             text not null,
  jd               text not null default '',
  duration_minutes int  not null default 90,
  expires_at       timestamptz not null,
  used             boolean not null default false,
  created_at       timestamptz not null default now()
);

create table if not exists v2.sessions (
  id               text primary key,                 -- ses_<rand>
  token            text not null unique,             -- session bearer token
  assessment_title text not null default '',
  role             text not null,
  jd_hash          text not null default '',
  product_slug     text not null,
  bank_task_id     text not null,                    -- which _tasks/<id> the overlay comes from
  task_version     text not null,
  brief            text not null default '',         -- the resolved TASK.md
  status           text not null default 'ready'
                   check (status in ('ready', 'active', 'submitted', 'expired', 'revoked')),
  started_at       timestamptz not null default now(),
  ends_at          timestamptz not null,
  created_at       timestamptz not null default now()
);

create index if not exists sessions_token_idx on v2.sessions (token);

alter table v2.setup_codes disable row level security;
alter table v2.sessions    disable row level security;

insert into v2.schema_migrations (version) values ('0009_sessions')
  on conflict (version) do nothing;
