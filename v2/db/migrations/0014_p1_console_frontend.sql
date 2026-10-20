-- P1 gained a web console (console/), so it now assesses the Frontend role and
-- its Full-Stack coverage spans the UI. The deployed selector reads coverage
-- from v2.products.role_coverage (not the repo manifest), so update it here.
-- Idempotent: re-running just re-sets the same JSON.
--
-- golden_commit is intentionally left unchanged: the console is purely additive
-- (no backend file changed), so cached backend task resolutions stay valid.
-- Run this after deploying the commit that adds console/, then the engine will
-- resolve the two new frontend tasks from the repo tree.

update v2.products
set
  stack = '["go", "react", "typescript"]'::jsonb,
  role_coverage = $manifest$
{
  "tags": ["support", "helpdesk", "tickets", "ticketing", "customer", "sla", "saas", "b2b", "csat", "knowledge", "webhooks", "react", "typescript", "javascript", "html", "css", "frontend", "ui", "components", "console", "vite"],
  "roles": {
    "frontend": {
      "subsystems": ["console/src/priority.ts", "console/src/sla.ts", "console/src/components/SlaBadge.tsx", "console/src/components/TicketList.tsx", "console/src/App.tsx"],
      "competencies": ["React component design", "typed state and props", "pure rendering logic", "API integration", "accessible markup", "front-end testing with Vitest"],
      "modes": ["implementation", "debugging", "testing"],
      "cannot_assess": ["design systems and visual polish (the console is intentionally unstyled)", "complex client-side routing or global state (the app is a single queue view)"]
    },
    "backend": {
      "subsystems": ["internal/support/store.go", "internal/support/tickets.go", "internal/support/sla.go", "internal/support/routing.go", "internal/support/webhooks.go", "internal/httpapi/"],
      "competencies": ["data modelling", "service design", "correctness", "reliability: retries and backoff", "HTTP API design"],
      "modes": ["implementation", "debugging", "testing", "delivery"],
      "cannot_assess": ["distributed systems at scale", "real database and query tuning (the store is in-memory)"]
    },
    "fullstack": {
      "subsystems": ["internal/httpapi/", "internal/support/", "console/src/"],
      "competencies": ["a vertical slice from the Go API through to the React console", "request validation", "JSON contract design", "wiring a UI to an API"],
      "modes": ["implementation", "testing", "delivery"],
      "cannot_assess": ["database and query tuning (the store is in-memory)"]
    },
    "fde": {
      "subsystems": ["internal/support/suggest.go", "internal/support/sla.go", "internal/seed/", "internal/httpapi/"],
      "competencies": ["investigate an unfamiliar data model and API", "triage by SLA breach", "integrate the suggested-reply feature", "customer-facing judgement"],
      "modes": ["implementation", "debugging", "research", "communication"],
      "cannot_assess": ["the live simulated-customer conversation (handled by the platform panel, not this codebase)"]
    },
    "ai_engineer": {
      "subsystems": ["internal/support/suggest.go"],
      "competencies": ["retrieval and ranking", "relevance tuning", "evaluating suggestion quality"],
      "modes": ["implementation", "testing"],
      "cannot_assess": ["model training or serving (retrieval here is deterministic, no model call)"]
    },
    "data_analyst": {
      "subsystems": ["internal/support/metrics.go"],
      "competencies": ["metric definitions", "SLA-compliance computation", "CSAT and resolution-time analytics"],
      "modes": ["implementation", "testing", "research"],
      "cannot_assess": ["warehouse-scale SQL (aggregation is in-memory here)"]
    }
  }
}
$manifest$::jsonb
where slug = 'p1-support-saas';
