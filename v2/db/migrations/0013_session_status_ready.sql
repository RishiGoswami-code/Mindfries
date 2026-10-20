-- Fix: 0012 made 'ready' the default session status, but the CHECK constraint
-- created in 0009 only allowed ('active','submitted','expired','revoked'), so
-- exchange (which inserts status 'ready') failed with a constraint violation.
-- Allow 'ready'. Safe to re-run.

alter table v2.sessions drop constraint if exists sessions_status_check;
alter table v2.sessions add constraint sessions_status_check
  check (status in ('ready', 'active', 'submitted', 'expired', 'revoked'));

insert into v2.schema_migrations (version) values ('0013_session_status_ready')
  on conflict (version) do nothing;
