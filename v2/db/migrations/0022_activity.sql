-- B1.6: the tracker. Every notification, flag and activity the panel shows is
-- a row here: derived from the CLI's events, the panel's own reports and
-- the server. A flag is a prompt for a reviewer to look, never a verdict
-- (SPEC section 16). Run after 0017.

create table if not exists v2.activity (
  id          bigserial primary key,                   -- also the SSE event id
  session_id  text not null references v2.sessions(id) on delete cascade,
  at          timestamptz not null default now(),
  severity    text not null check (severity in ('info', 'notice', 'flag')),
  source      text not null check (source in ('watcher', 'panel', 'server')),
  category    text check (category in ('observed', 'verified', 'signal')),
  kind        text not null default '',                -- the rule or event that made it
  text        text not null,
  event_seq   bigint,                                  -- the CLI event it came from, if any
  evidence_id text,                                    -- a still, screenshot or clip, if any
  data        jsonb not null default '{}'::jsonb,
  -- unique per (session, kind, event_seq) so re-sent events don't double up
  dedupe_key  text
);
create index if not exists activity_session_idx on v2.activity (session_id, id);
create unique index if not exists activity_dedupe_idx on v2.activity (session_id, dedupe_key) where dedupe_key is not null;

-- What a reviewer did about a flag.
create table if not exists v2.flag_reviews (
  activity_id bigint primary key references v2.activity(id) on delete cascade,
  decision    text not null check (decision in ('noted', 'explained', 'dismissed', 'escalated')),
  note        text not null default '',
  reviewer    text not null default '',
  reviewed_at timestamptz not null default now()
);

alter table v2.activity     disable row level security;
alter table v2.flag_reviews disable row level security;

insert into v2.schema_migrations (version) values ('0022_activity')
  on conflict (version) do nothing;
