-- B1.8: the assistant. `mindfries agent` and the AI avatar share one record
-- per session, so a reviewer reads one history. A turn is one prompt and the
-- model's whole reply; steps are what it did (say something, call a tool).
-- Token use is kept for the budgets and the cost caps. Run after 0017.

create table if not exists v2.assistant_turns (
  id           text primary key,                      -- trn_<rand>
  session_id   text not null references v2.sessions(id) on delete cascade,
  channel      text not null check (channel in ('agent', 'avatar')),
  prompt       text not null default '',
  status       text not null default 'running'
               check (status in ('running', 'waiting_tools', 'completed', 'failed', 'cancelled')),
  started_at   timestamptz not null default now(),
  ended_at     timestamptz,
  input_tokens  int not null default 0,
  output_tokens int not null default 0,
  error_code   text
);
create index if not exists assistant_turns_session_idx on v2.assistant_turns (session_id, started_at);

create table if not exists v2.assistant_steps (
  turn_id    text not null references v2.assistant_turns(id) on delete cascade,
  step       int  not null,
  kind       text not null check (kind in ('text', 'tool_call', 'tool_result', 'transcript_in', 'transcript_out')),
  tool       text,
  body       jsonb not null default '{}'::jsonb,
  at         timestamptz not null default now(),
  primary key (turn_id, step)
);

-- A model call's cost, for the daily cap and the admin portal.
create table if not exists v2.model_usage (
  id            bigserial primary key,
  session_id    text references v2.sessions(id) on delete set null,
  purpose       text not null,                         -- agent | avatar | evaluator | task
  model         text not null,
  input_tokens  int not null default 0,
  output_tokens int not null default 0,
  at            timestamptz not null default now()
);
create index if not exists model_usage_at_idx on v2.model_usage (at);

alter table v2.assistant_turns disable row level security;
alter table v2.assistant_steps disable row level security;
alter table v2.model_usage     disable row level security;

insert into v2.schema_migrations (version) values ('0024_assistant')
  on conflict (version) do nothing;
