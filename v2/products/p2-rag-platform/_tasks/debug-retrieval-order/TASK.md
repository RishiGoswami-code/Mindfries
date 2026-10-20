# Fix: retrieval returns the least relevant results first

Users report that answers cite irrelevant documents. Investigation points at
retrieval: a query for "reset my password" is surfacing billing and export
passages ahead of the login article. Relevance scoring looks right, but the
results come back in the wrong order.

## Your task

Find and fix the bug so the most similar chunks are returned first. Keep tie
behaviour deterministic.

## How to check

```sh
go test ./internal/rag/
```

The retrieval and eval tests currently fail because the top result is the least
relevant one. Make them pass.

## Where to look

Nearest-neighbour search lives in `internal/rag/store.go`. Embedding and
similarity are in `embed.go`; the chunking/ingest path is in `chunk.go` and
`engine.go`.
