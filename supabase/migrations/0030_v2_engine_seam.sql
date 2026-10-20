-- v1-seam: the ONE V1 change V2 needs. Additive and switch-guarded — every
-- existing row defaults to engine 'v1', so V1 behaves exactly as today. V2 is
-- opted into per assessment by setting engine='v2' (+ the job description the
-- engine selects a product/task from).
--
-- Run this only when wiring the company portal to offer V2. Rollback is simply
-- leaving engine at 'v1' (or dropping the columns).

alter table assessments
  add column if not exists engine text not null default 'v1',
  add column if not exists jd     text;

-- A V2 assessment: the candidate app routes its Start/report to the V2 flow,
-- and the engine resolves a product + task from (role, jd). Until an assessment
-- is explicitly set to 'v2', nothing changes.
--   update assessments set engine='v2', jd='<job description>' where id='<id>';
