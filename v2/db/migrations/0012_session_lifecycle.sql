-- Two-phase session lifecycle (PR #172): a session is created in status
-- 'ready' at setup, then begin starts the clock (status 'active'). duration is
-- carried so begin can compute ends_at. Run after 0009.

alter table v2.sessions add column if not exists duration_minutes int not null default 90;

-- New sessions default to 'ready'; the clock (started_at/ends_at) is set on begin.
alter table v2.sessions alter column status set default 'ready';

insert into v2.schema_migrations (version) values ('0012_session_lifecycle')
  on conflict (version) do nothing;
