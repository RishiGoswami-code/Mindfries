# P2 — AI RAG/Agent platform

One of the owned product codebases candidates are assessed on
(`v2/codebase-engine/ANALYSIS.md`). A retrieval-augmented question-answering
platform: ingest documents, chunk and embed them, retrieve by similarity,
answer with a tool-calling agent, and measure quality with an evaluation
harness. Candidates work on this real product; each assessment targets one
role's subsystem (`manifest.json`).

## Run it

```sh
go run ./cmd/server          # listens on :8080
PORT=9000 go run ./cmd/server
```

Endpoints: `GET /healthz`, `POST /documents` (ingest), `GET /query?q=…&k=3`,
`POST /answer` (`{"q":"…"}`), `POST /eval` (`{"cases":[…],"k":3}`). The server
seeds a small knowledge base on start.

## Test it

```sh
go test ./...        # the suite that defines "correct" for this product
go vet ./...
```

No network or model needed — the embedder is a deterministic local stand-in
(`internal/rag/embed.go`): a sparse term-frequency vector with stop-word
removal and light stemming. A real embedding model plugs in behind the same
`Vector` type, which is itself good assessment material.

## Layout

```
cmd/server/            runnable entry point (seeds a KB, serves the API)
internal/rag/          the engine
  doc.go               Document, Chunk, Vector, Hit
  chunk.go             word-window chunking with overlap
  embed.go             the model-free embedder + cosine similarity
  store.go             in-memory vector store + nearest-neighbour search
  engine.go            ingest (chunk+embed) and query
  agent.go             tool-calling agent (calculator + retrieval fallback)
  eval.go              evaluation harness (HitAtK, AnswerMatch)
internal/httpapi/      thin JSON HTTP layer
internal/seed/         sample knowledge base + evaluation set
manifest.json          role-coverage manifest
_tasks/                reference assessment tasks (feature + debug)
```

The `_tasks/` directory is named with a leading underscore so the Go tool
ignores it: a candidate's delivered workspace excludes it anyway, and
`go test ./...` at the product root stays clean.

## What it assesses

See `manifest.json`. This build assesses **AI-Engineer** (chunking, embedding,
retrieval/ranking, agent orchestration), **AI/ML-Researcher** (the eval
harness), and **Backend** (ingestion + serving API).

> **Not yet in this build:** a chat web UI (Frontend coverage) — deferred like
> P1's console; the manifest claims only what the code can assess.
