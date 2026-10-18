-- Mindfries — Allow company to share assessment report with candidate.
--
-- Reports remain private to the company side until shared_with_candidate is true.
-- When a candidate finishes an assessment/interview, they see an "Under Review"
-- confirmation screen. The evaluation, recommendation, and observations are only
-- released to the candidate when the company explicitly chooses to share the report.
--
-- Additive only. Apply after 0027_company_self_signup_and_credits.sql.

alter table assessment_reports
  add column if not exists shared_with_candidate boolean not null default false,
  add column if not exists shared_at timestamptz,
  add column if not exists shared_by text;

create index if not exists assessment_reports_shared_idx on assessment_reports (session_id, shared_with_candidate);
