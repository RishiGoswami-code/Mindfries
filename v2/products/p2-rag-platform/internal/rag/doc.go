// Package rag is the domain of the AI RAG/Agent platform: ingest documents,
// chunk and embed them, retrieve by similarity, answer questions with a
// tool-calling agent, and evaluate retrieval/answer quality. The HTTP API and
// server are thin layers over this package.
//
// The embedder here is a deterministic local stand-in (embed.go), not a model,
// so the product is hermetic and its tests need no network. In production a
// real embedding model plugs in behind the same Vector interface — which is
// itself good assessment material (swap or improve the embedder, tune
// retrieval, extend the eval harness).
package rag

// Document is a source document to make retrievable.
type Document struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Chunk is a passage of a document, the unit that gets embedded and retrieved.
type Chunk struct {
	ID    string `json:"id"` // "<docID>#<index>"
	DocID string `json:"doc_id"`
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// Vector is an embedding: a sparse map from token to weight (see embed.go).
type Vector map[string]float64

// Hit is a retrieved chunk with its similarity score.
type Hit struct {
	Chunk Chunk   `json:"chunk"`
	Score float64 `json:"score"`
}
