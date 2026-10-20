-- B1.4: the monitoring panel. What it reports (camera, screen, microphone),
-- the stills, screenshots and audio clips it uploads (the bytes live in
-- Storage; here is where they are), and the gaps when a share stops. A gap is
-- a gap in the record, never a verdict. Run after 0017.

create table if not exists v2.monitoring_state (
  session_id   text primary key references v2.sessions(id) on delete cascade,
  panel_open   boolean not null default false,
  camera       boolean not null default false,
  screen       boolean not null default false,
  audio        boolean not null default false,
  reported_at  timestamptz not null default now()
);

create table if not exists v2.evidence_objects (
  id           text primary key,                    -- evi_<rand>
  session_id   text not null references v2.sessions(id) on delete cascade,
  kind         text not null check (kind in ('camera_still', 'screenshot', 'audio_clip', 'avatar_audio', 'attachment')),
  taken_at     timestamptz not null,
  storage_path text not null,                       -- path inside the bucket
  bytes        bigint not null default 0,
  sha256       text not null default '',
  content_type text not null default '',
  expires_at   timestamptz,                         -- retention (B12)
  deleted_at   timestamptz,
  created_at   timestamptz not null default now()
);
create index if not exists evidence_session_idx on v2.evidence_objects (session_id, taken_at);
create index if not exists evidence_expiry_idx  on v2.evidence_objects (expires_at) where deleted_at is null;

create table if not exists v2.monitoring_gaps (
  id          bigserial primary key,
  session_id  text not null references v2.sessions(id) on delete cascade,
  source      text not null check (source in ('camera', 'screen', 'audio', 'panel')),
  started_at  timestamptz not null,
  ended_at    timestamptz,                          -- null while it is still off
  reason      text not null default ''
);
create index if not exists gaps_session_idx on v2.monitoring_gaps (session_id, started_at);
-- at most one open gap per source
create unique index if not exists gaps_open_idx on v2.monitoring_gaps (session_id, source) where ended_at is null;

alter table v2.monitoring_state disable row level security;
alter table v2.evidence_objects disable row level security;
alter table v2.monitoring_gaps  disable row level security;

insert into v2.schema_migrations (version) values ('0020_monitoring')
  on conflict (version) do nothing;
