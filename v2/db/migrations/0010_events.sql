-- Session evidence: watcher events (hash-chained) and git-diff snapshots.
-- Stored as received; integrity/evaluation reads them later. Internal to the
-- v2_api server, so RLS is disabled (see 0008). Run after 0009.

create table if not exists v2.events (
  session_id  text not null references v2.sessions(id) on delete cascade,
  seq         bigint not null,
  occurred_at timestamptz not null,
  type        text not null,
  data        jsonb not null default '{}'::jsonb,
  prev_hash   text not null default '',   -- SHA-256 of the previous event's hash
  hash        text not null default '',   -- SHA-256 of {seq,occurred_at,type,data,prev_hash}
  received_at timestamptz not null default now(),
  primary key (session_id, seq)           -- idempotent by (session, seq)
);

create table if not exists v2.snapshots (
  session_id  text not null references v2.sessions(id) on delete cascade,
  seq         bigint not null,
  taken_at    timestamptz not null,
  base_commit text not null default '',
  head_commit text not null default '',
  patch       bytea,                      -- gzipped git diff
  patch_sha256 text not null default '',  -- SHA-256 of the uncompressed patch
  received_at timestamptz not null default now(),
  primary key (session_id, seq)
);

alter table v2.events    disable row level security;
alter table v2.snapshots disable row level security;

insert into v2.schema_migrations (version) values ('0010_events')
  on conflict (version) do nothing;
