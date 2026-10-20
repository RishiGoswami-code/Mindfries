-- B1.2: the browser sign-in (RFC 8628 device authorization). `mindfries start`
-- asks for a device code, the candidate approves it on the portal's sign-in
-- page, and the CLI polls until it is approved. Run after 0017.

create table if not exists v2.device_codes (
  device_code_hash text primary key,          -- SHA-256 of the secret the CLI polls with
  user_code        text not null unique,      -- the short code shown on the page
  expires_at       timestamptz not null,
  interval_seconds int not null default 5,
  last_polled_at   timestamptz,
  status           text not null default 'pending'
                   check (status in ('pending', 'approved', 'denied', 'used')),
  candidate_id     uuid,                      -- set on approval
  device_info      jsonb,                     -- {os, os_version, arch}: shown on the sign-in page
  grant_data       jsonb,                     -- what approval entitles the CLI to (session.Grant)
  session_id       text references v2.sessions(id) on delete set null,
  approved_at      timestamptz,
  created_at       timestamptz not null default now()
);

alter table v2.device_codes add column if not exists grant_data jsonb;
alter table v2.device_codes add column if not exists device_info jsonb;

create index if not exists device_codes_expires_idx on v2.device_codes (expires_at);
alter table v2.device_codes disable row level security;

insert into v2.schema_migrations (version) values ('0018_device_codes')
  on conflict (version) do nothing;
