-- v1-seam: V2 assessments set task_status='none' (no V1 task generation — the
-- worker only picks up 'pending'/'generating'). The 0022 check didn't allow
-- 'none', so inserting a V2 assessment failed. Widen it. Additive and safe.

alter table assessments drop constraint if exists assessments_task_status_check;
alter table assessments add constraint assessments_task_status_check
  check (task_status in ('pending', 'generating', 'ready', 'failed', 'none'));
