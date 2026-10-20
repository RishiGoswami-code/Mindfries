package rag

import "strings"

// Chunk splits text into word windows of size words, each overlapping the
// previous by overlap words. Overlap keeps context from being severed at a
// boundary. size must be > 0 and overlap must be < size; otherwise the whole
// text is returned as one chunk.
func ChunkText(text string, size, overlap int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if size <= 0 || overlap < 0 || overlap >= size {
		return []string{strings.Join(words, " ")}
	}
	step := size - overlap
	var out []string
	for start := 0; start < len(words); start += step {
		end := start + size
		if end > len(words) {
			end = len(words)
		}
		out = append(out, strings.Join(words[start:end], " "))
		if end == len(words) {
			break
		}
	}
	return out
}
