// Package engine is the heart of the redesign: pick an owned product for a
// (role, JD) and resolve the task to run on it. This file handles the JD cache
// key — the thing that makes "generate once, reuse across candidates, regen
// only when the company edits the JD" work (ARCHITECTURE part 3).
package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// NormalizeJD canonicalizes a job description so trivial edits (whitespace,
// case, punctuation) don't invalidate the cache, while a real content change
// does. Lowercase, punctuation to spaces, collapse runs of whitespace.
func NormalizeJD(jd string) string {
	var b strings.Builder
	b.Grow(len(jd))
	prevSpace := false
	for _, r := range jd {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		default:
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// HashJD is the cache key component for a JD: sha256 of the normalized text.
// Same JD (ignoring formatting) -> same hash -> cached task pool is reused.
func HashJD(jd string) string {
	sum := sha256.Sum256([]byte(NormalizeJD(jd)))
	return hex.EncodeToString(sum[:])
}

// jdTokens returns the unique word tokens of a JD, for keyword matching.
func jdTokens(jd string) map[string]bool {
	toks := map[string]bool{}
	for _, w := range strings.Fields(NormalizeJD(jd)) {
		toks[w] = true
	}
	return toks
}

// TagScore counts how many of tags the JD mentions, returning the score and the
// tags that hit. Shared by product selection and task-bank ranking.
func TagScore(jd string, tags []string) (int, []string) {
	toks := jdTokens(jd)
	score := 0
	var hits []string
	for _, tag := range tags {
		if toks[strings.ToLower(tag)] {
			score++
			hits = append(hits, tag)
		}
	}
	return score, hits
}
