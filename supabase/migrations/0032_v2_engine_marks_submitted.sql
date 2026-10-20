-- The V2 engine (role v2_api) can only READ the V1 public tables. For the
-- resolve→work→submit lifecycle to close, the engine must flip an assessment to
-- 'submitted' when the candidate hands in via the CLI. Grant exactly that — a
-- column-scoped UPDATE on assessments.status, nothing more.
--
-- Run after v2/db/migrations/0016 (which adds the assessment_id link columns).
-- Safe to re-run.
grant update (status) on table public.assessments to v2_api;
