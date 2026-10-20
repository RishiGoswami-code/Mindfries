-- Register product P2 (AI RAG/Agent platform). Run after 0001–0003.
-- Idempotent: upserts on slug. Bump golden_commit/version when the product
-- source changes.

insert into v2.products (slug, name, domain, repo_ref, golden_commit, stack, role_coverage, status, version)
values (
  'p2-rag-platform',
  'AI RAG/Agent platform',
  'retrieval augmented generation agent llm ai',
  'v2/products/p2-rag-platform',
  'bf089314d50403b8cbc7646633690962ed1ede8a',
  '["go"]'::jsonb,
  $manifest$
  {
    "tags": ["rag", "retrieval", "agent", "llm", "ai", "embeddings", "embedding", "vector", "chatbot", "knowledge", "eval", "evaluation", "chunking", "semantic"],
    "roles": {
      "ai_engineer": {
        "subsystems": ["internal/rag/chunk.go", "internal/rag/embed.go", "internal/rag/store.go", "internal/rag/engine.go", "internal/rag/agent.go"],
        "competencies": ["document chunking and embedding", "vector retrieval and ranking", "tool-calling agent orchestration", "relevance tuning"],
        "modes": ["implementation", "debugging", "testing"],
        "cannot_assess": ["training or hosting a real model (the embedder here is a deterministic stand-in)"]
      },
      "ml_researcher": {
        "subsystems": ["internal/rag/eval.go"],
        "competencies": ["evaluation design", "retrieval and answer metrics", "reproducible measurement", "leakage awareness"],
        "modes": ["implementation", "research", "testing"],
        "cannot_assess": ["large-scale training experiments or GPU workloads"]
      },
      "backend": {
        "subsystems": ["internal/rag/engine.go", "internal/httpapi/"],
        "competencies": ["ingestion pipeline", "service and API design", "JSON contract design"],
        "modes": ["implementation", "testing", "delivery"],
        "cannot_assess": ["real vector-database operations (the store is in-memory)"]
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
