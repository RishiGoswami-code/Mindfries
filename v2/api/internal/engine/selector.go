// Product selection (ARCHITECTURE part 2): role + JD -> one owned product.
// No codebase is generated. The pick is deterministic and cheap: filter to
// products that cover the role, then rank by how many of the product's domain
// tags the JD mentions. Deliberately keyword-based, not embedding-based, so
// launch needs no model call and the choice is auditable; an embedding ranker
// can slot in behind the same Select signature later if tags prove too blunt.
package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/manifest"
)

// Product is the registry row the selector needs (v2.products).
type Product struct {
	Slug         string
	Name         string
	Domain       string
	GoldenCommit string
	Manifest     manifest.Manifest
}

// Choice is a selection result, with the score for audit.
type Choice struct {
	Product Product
	Score   int    // tag hits in the JD (+ domain-word hits)
	Why     string // short human-readable reason
}

// Select picks the best product for (role, jd) from products. It errors only
// when no product covers the role — a product gap worth surfacing loudly, not
// papering over with a bad match.
func Select(role core.Role, jd string, products []Product) (Choice, error) {
	if !core.ValidRole(role) {
		return Choice{}, fmt.Errorf("select: unknown role %q", role)
	}

	toks := jdTokens(jd)

	var candidates []Choice
	for _, p := range products {
		if !p.Manifest.Covers(role) {
			continue
		}
		score, hits := scoreProduct(p, toks)
		why := fmt.Sprintf("covers %s", role)
		if len(hits) > 0 {
			why += "; JD matches " + strings.Join(hits, ", ")
		} else {
			why += "; no JD keyword match (role-only pick)"
		}
		candidates = append(candidates, Choice{Product: p, Score: score, Why: why})
	}

	if len(candidates) == 0 {
		return Choice{}, fmt.Errorf("select: no product covers role %q", role)
	}

	// Highest score wins; ties break by slug so the choice is stable and
	// reproducible across runs.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Product.Slug < candidates[j].Product.Slug
	})
	return candidates[0], nil
}

// scoreProduct counts JD mentions of the product's tags and domain words.
func scoreProduct(p Product, jdToks map[string]bool) (int, []string) {
	var hits []string
	score := 0
	for _, tag := range p.Manifest.Tags {
		if jdToks[strings.ToLower(tag)] {
			score++
			hits = append(hits, tag)
		}
	}
	// Domain words count too, so a product with sparse tags still matches on
	// an obvious domain term ("payments", "support", ...).
	for _, w := range strings.Fields(NormalizeJD(p.Domain)) {
		if jdToks[w] {
			score++
		}
	}
	return score, hits
}
