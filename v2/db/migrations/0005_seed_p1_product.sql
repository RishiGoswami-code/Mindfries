-- Register product P1 (Support SaaS) in the registry. Run after 0001–0003.
-- Idempotent: upserts on slug, so re-running updates the manifest in place.
--
-- golden_commit is the monorepo commit the test-green product was frozen at;
-- bump it (and version) whenever the product source changes. repo_ref is the
-- path within this repo until products move to their own object store.

insert into v2.products (slug, name, domain, repo_ref, golden_commit, stack, role_coverage, status, version)
values (
  'p1-support-saas',
  'Support SaaS (Helpdesk)',
  'customer support helpdesk saas',
  'v2/products/p1-support-saas',
  'e0037896d96be0909543f9462e8cd3a46ab1b8bc',
  '["go"]'::jsonb,
  $manifest$
  {
    "tags": ["support", "helpdesk", "tickets", "ticketing", "customer", "sla", "saas", "b2b", "csat", "knowledge", "webhooks"],
    "roles": {
      "backend": {
        "subsystems": ["internal/support/store.go", "internal/support/tickets.go", "internal/support/sla.go", "internal/support/routing.go", "internal/support/webhooks.go", "internal/httpapi/"],
        "competencies": ["data modelling", "service design", "correctness", "reliability: retries and backoff", "HTTP API design"],
        "modes": ["implementation", "debugging", "testing", "delivery"],
        "cannot_assess": ["distributed systems at scale", "real database and query tuning (the store is in-memory)"]
      },
      "fullstack": {
        "subsystems": ["internal/httpapi/", "internal/support/"],
        "competencies": ["a vertical slice across API and domain", "request validation", "JSON contract design"],
        "modes": ["implementation", "testing", "delivery"],
        "cannot_assess": ["browser UI (no web console in this build yet)"]
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
  $manifest$::jsonb,
  'verified',
  1
)
on conflict (slug) do update set
  name = excluded.name,
  domain = excluded.domain,
  repo_ref = excluded.repo_ref,
  golden_commit = excluded.golden_commit,
  stack = excluded.stack,
  role_coverage = excluded.role_coverage,
  status = excluded.status,
  version = v2.products.version + 1,
  updated_at = now();
