# Add Recall@K to the evaluation harness

The eval harness reports `HitAtK` (did *any* relevant document appear in the
top-k?). For questions with several relevant documents that is too forgiving —
we want to know what fraction of the relevant set we actually retrieved.

## Your task

Implement `RecallAtK` in `internal/rag/recall.go`:

```go
func (e *Eval) RecallAtK(cases []Case, k int) float64
```

For each case, compute the fraction of its `RelevantDocIDs` that appear among
the distinct documents in the top-k retrieved chunks, then average over all
cases. A case with no relevant documents contributes 0. The result is in
`[0, 1]`.

A stub is provided so the package compiles; replace it.

## How to check

```sh
go test ./internal/rag/
```

Keep the rest of the package's tests green.
