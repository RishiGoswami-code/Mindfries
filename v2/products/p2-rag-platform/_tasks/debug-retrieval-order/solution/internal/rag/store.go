package rag

import "sort"

// VectorStore holds chunk embeddings and answers nearest-neighbour queries. In
// production this is a vector database; in-memory here keeps the suite
// dependency-free behind the same small interface.
type VectorStore struct {
	chunks []Chunk
	vecs   []Vector
}

// NewVectorStore returns an empty store.
func NewVectorStore() *VectorStore { return &VectorStore{} }

// Add stores a chunk and its embedding.
func (s *VectorStore) Add(c Chunk, v Vector) {
	s.chunks = append(s.chunks, c)
	s.vecs = append(s.vecs, v)
}

// Len is the number of stored chunks.
func (s *VectorStore) Len() int { return len(s.chunks) }

// Search returns the k chunks most similar to q, highest score first. Ties
// break by chunk id for deterministic ordering.
func (s *VectorStore) Search(q Vector, k int) []Hit {
	hits := make([]Hit, 0, len(s.chunks))
	for i, c := range s.chunks {
		hits = append(hits, Hit{Chunk: c, Score: Cosine(q, s.vecs[i])})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Chunk.ID < hits[j].Chunk.ID
	})
	if k > 0 && len(hits) > k {
		hits = hits[:k]
	}
	return hits
}
