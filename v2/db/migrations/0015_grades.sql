-- GRADE step: the objective result of running a submission against its task's
-- hidden acceptance tests. One row per submission, written out-of-process by an
-- isolated grading runner (never the live engine — it executes candidate code).
create table if not exists v2.grades (
  submission_id text primary key references v2.submissions(id) on delete cascade,
  passed        boolean     not null,
  check_cmd     jsonb       not null default '[]'::jsonb,  -- the command that was run
  output        text        not null default '',          -- combined check output
  graded_at     timestamptz not null default now()
);

-- Internal engine table: no PostgREST/RLS (matches the other v2.* tables).
alter table v2.grades disable row level security;
