-- B1.9: the retention log (B12): what was deleted, when and why. Evaluation
-- and the report are another team's work and have their own migrations; this
-- file deliberately has nothing of theirs. Run after 0017.

create table if not exists v2.deletion_log (
  id          bigserial primary key,
  session_id  text,
  what        text not null,                          -- evidence_media | transcripts | candidate_data
  reason      text not null,                          -- retention | candidate_request
  count       int not null default 0,
  at          timestamptz not null default now()
);

alter table v2.deletion_log      disable row level security;

insert into v2.schema_migrations (version) values ('0025_deletion_log')
  on conflict (version) do nothing;
