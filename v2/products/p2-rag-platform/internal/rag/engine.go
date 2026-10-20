package rag

import "fmt"

// Engine ties ingestion and retrieval together: it chunks and embeds documents
// into the store, and answers similarity queries.
type Engine struct {
	store   *VectorStore
	size    int
	overlap int
}

// NewEngine returns an engine with the given chunking parameters (sensible
// defaults applied if non-positive).
func NewEngine(size, overlap int) *Engine {
	if size <= 0 {
		size = 40
	}
	if overlap < 0 || overlap >= size {
		overlap = 10
	}
	return &Engine{store: NewVectorStore(), size: size, overlap: overlap}
}

// Ingest chunks, embeds, and stores a document.
func (e *Engine) Ingest(d Document) {
	for i, text := range ChunkText(d.Text, e.size, e.overlap) {
		c := Chunk{ID: fmt.Sprintf("%s#%d", d.ID, i), DocID: d.ID, Index: i, Text: text}
		e.store.Add(c, Embed(text))
	}
}

// Query embeds q and returns the top-k chunks.
func (e *Engine) Query(q string, k int) []Hit {
	return e.store.Search(Embed(q), k)
}

// Store exposes the underlying store (read-only use by callers/tests).
func (e *Engine) Store() *VectorStore { return e.store }
