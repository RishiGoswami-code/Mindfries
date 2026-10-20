-- B1.3: the candidate's GitHub. One connection per candidate (the token is
-- encrypted by the server with V2_ENCRYPTION_KEY, never stored plain), and the
-- private repository created for each session. Run after 0017.

create table if not exists v2.github_connections (
  candidate_id    uuid primary key,
  github_login    text not null,
  github_id       bigint not null,
  token_encrypted bytea,                    -- AES-256-GCM, nonce first; wiped once collected
  scopes          text not null default '',
  connected_at    timestamptz not null default now(),
  revoked_at      timestamptz
);

-- A one-time state value for the OAuth round trip.
create table if not exists v2.github_oauth_states (
  state        text primary key,            -- random, in the redirect and the callback
  candidate_id uuid,
  session_id   text,
  return_to    text not null default '',
  expires_at   timestamptz not null,
  used         boolean not null default false,
  result       text not null default '' check (result in ('', 'connected', 'failed')),
  error        text not null default '',
  created_at   timestamptz not null default now()
);
alter table v2.github_connections alter column token_encrypted drop not null;
alter table v2.github_oauth_states add column if not exists result text not null default '';
alter table v2.github_oauth_states add column if not exists error text not null default '';
alter table v2.github_oauth_states add column if not exists created_at timestamptz not null default now();

create index if not exists github_states_session_idx on v2.github_oauth_states (session_id, created_at);

-- The token is handed to the CLI once per session, then forgotten.
create table if not exists v2.github_handoffs (
  session_id   text primary key references v2.sessions(id) on delete cascade,
  collected_at timestamptz not null default now()
);

create table if not exists v2.session_repos (
  session_id   text primary key references v2.sessions(id) on delete cascade,
  owner        text not null,
  name         text not null,
  html_url     text not null default '',
  private      boolean not null default true,
  invited      jsonb not null default '[]'::jsonb,   -- reviewers invited: [{login, ok, detail}]
  created_at   timestamptz not null default now()
);

alter table v2.github_connections  disable row level security;
alter table v2.github_oauth_states disable row level security;
alter table v2.session_repos       disable row level security;
alter table v2.github_handoffs     disable row level security;

insert into v2.schema_migrations (version) values ('0019_github')
  on conflict (version) do nothing;
