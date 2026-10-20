-- B1.7: what the candidate makes in the panel: the system-design diagram, notes,
-- code-review comments and debugging hypotheses. One current row per artifact
-- (kind) per session, with every saved version kept as evidence. Test and
-- check runs the CLI reports are kept here too. Run after 0017.

create table if not exists v2.artifacts (
  session_id text not null references v2.sessions(id) on delete cascade,
  kind       text not null check (kind in ('diagram', 'notes', 'review', 'hypotheses')),
  version    int  not null default 1,
  body       jsonb not null,
  updated_at timestamptz not null default now(),
  primary key (session_id, kind)
);

create table if not exists v2.artifact_versions (
  session_id text not null references v2.sessions(id) on delete cascade,
  kind       text not null,
  version    int  not null,
  body       jsonb not null,
  saved_at   timestamptz not null default now(),
  primary key (session_id, kind, version)
);

create table if not exists v2.check_runs (
  id          bigserial primary key,
  session_id  text not null references v2.sessions(id) on delete cascade,
  event_seq   bigint,
  ran_at      timestamptz not null,
  command     text not null,
  passed      boolean not null,
  failing     jsonb not null default '[]'::jsonb,       -- test names
  passing     jsonb not null default '[]'::jsonb,
  output_tail text not null default ''
);
create unique index if not exists check_runs_event_idx on v2.check_runs (session_id, event_seq) where event_seq is not null;

alter table v2.artifacts         disable row level security;
alter table v2.artifact_versions disable row level security;
alter table v2.check_runs        disable row level security;

insert into v2.schema_migrations (version) values ('0023_panel_artifacts')
  on conflict (version) do nothing;
