-- Link a V2 session back to the V1 assessments row it belongs to, so the engine
-- can reflect progress (submission) onto assessments.status — otherwise a
-- submitted assessment keeps showing "Set up workspace" / "not started" and the
-- candidate can set up again (multiple sessions). Nullable: a code may be
-- minted without a link (older flow). Idempotent.
alter table v2.setup_codes add column if not exists assessment_id text;
alter table v2.sessions    add column if not exists assessment_id text;
