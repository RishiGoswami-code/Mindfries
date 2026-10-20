package rag

import (
	"math"
	"strings"
	"unicode"
)

// Embed maps text to a sparse term-frequency vector (token -> normalized count)
// with common stop words removed. It is a deterministic, model-free stand-in:
// collision-free (unlike a hashing embedder) so retrieval is accurate, and
// simple enough that improving it — IDF weighting, real embeddings — is itself
// good assessment material. A real model plugs in behind the same Vector type.
func Embed(text string) Vector {
	tf := Vector{}
	for _, tok := range tokenize(text) {
		tf[tok]++
	}
	var sum float64
	for _, v := range tf {
		sum += v * v
	}
	if sum > 0 {
		norm := math.Sqrt(sum)
		for k := range tf {
			tf[k] /= norm
		}
	}
	return tf
}

// Cosine is the cosine similarity of two unit TF vectors.
func Cosine(a, b Vector) float64 {
	if len(a) > len(b) {
		a, b = b, a // iterate the smaller map
	}
	var dot float64
	for k, va := range a {
		dot += va * b[k]
	}
	return dot
}

func tokenize(s string) []string {
	var toks []string
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len(f) >= 2 && !stopWords[f] {
			toks = append(toks, stem(f))
		}
	}
	return toks
}

// stem does naive plural folding so "refunds" and "refund" match. It trims a
// trailing "s" from words longer than three letters that don't end in "ss"
// (so "access" is untouched). A real stemmer would plug in here.
func stem(w string) string {
	if len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		return w[:len(w)-1]
	}
	return w
}

// stopWords are high-frequency words that carry little retrieval signal and
// otherwise cause cross-talk between documents.
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "you": true, "your": true,
	"with": true, "this": true, "that": true, "have": true, "not": true, "can": true,
	"but": true, "from": true, "was": true, "how": true, "any": true, "all": true,
	"our": true, "out": true, "use": true, "get": true, "within": true, "when": true,
	"second": true, "step": true, "least": true, "must": true, "from.": true,
}
