-- A ledger of every model call the backend makes, so AI spend is visible by
-- agent, by role, by company — not only per session.
--
-- Session-scoped calls are already recorded on the session's own trail as
-- 'ai_cost' activity_events (orchestrator.recordUsage), which is how per-
-- candidate spend is read. But task generation belongs to no session: it runs
-- for a role (role_tasks.go) or an invitation (invitation_tasks.go), so its
-- usage had nowhere to go and was dropped — which is why task generation never
-- showed up when adding spend up. This table catches every call, including
-- those, so it's visible which role or task is exhausting the API.
--
-- Append-only, no foreign keys: usage history should outlive the role or
-- invitation it was for, and a deleted role must never fail a usage write.

create table if not exists ai_usage (
  id                bigserial primary key,
  created_at        timestamptz not null default now(),
  -- Which agent made the call: 'task_generation', 'assistant', 'report', …
  agent             text not null default '',
  model             text not null default '',
  prompt_tokens     int  not null default 0,
  -- Includes reasoning/thinking tokens — they are billed as output.
  completion_tokens int  not null default 0,
  -- Of prompt_tokens, how many were served from cache (already in prompt_tokens).
  cached_tokens     int  not null default 0,
  -- Zero on Gemini, which doesn't price each call; the tokens are exact.
  cost_usd          numeric not null default 0,
  -- What the call was for. A session call has session_id; a task-generation
  -- call has role_id and/or invitation_id. company_id is set when known.
  session_id        uuid,
  company_id        uuid,
  role_id           uuid,
  invitation_id     uuid
);

create index if not exists ai_usage_created_idx    on ai_usage (created_at);
create index if not exists ai_usage_agent_idx      on ai_usage (agent);
create index if not exists ai_usage_role_idx       on ai_usage (role_id)       where role_id is not null;
create index if not exists ai_usage_company_idx    on ai_usage (company_id)    where company_id is not null;
create index if not exists ai_usage_invitation_idx on ai_usage (invitation_id) where invitation_id is not null;
