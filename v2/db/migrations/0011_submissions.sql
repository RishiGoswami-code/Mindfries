-- Candidate submissions: the handed-in manifest and the uploaded git bundle.
-- Internal to the v2_api server, RLS disabled (see 0008). Run after 0009.

create table if not exists v2.submissions (
  id                text primary key,                 -- sub_<rand>
  session_id        text not null references v2.sessions(id) on delete cascade,
  task_version      text not null default '',
  branch            text not null default '',
  base_commit       text not null default '',
  head_commit       text not null default '',
  commit_message    text not null default '',
  bundle_sha256     text not null default '',          -- SHA-256 the uploaded bundle must match
  bundle_bytes      bigint not null default 0,
  task_files_intact boolean not null default false,    -- locked files unchanged (candidate-reported)
  manifest          jsonb not null default '{}'::jsonb,-- the full submission manifest as received
  status            text not null default 'awaiting_bundle'
                    check (status in ('awaiting_bundle', 'verified', 'rejected')),
  bundle            bytea,                              -- the uploaded git bundle (null until uploaded)
  created_at        timestamptz not null default now(),
  verified_at       timestamptz
);

create unique index if not exists submissions_session_idx on v2.submissions (session_id);

alter table v2.submissions disable row level security;

insert into v2.schema_migrations (version) values ('0011_submissions')
  on conflict (version) do nothing;
