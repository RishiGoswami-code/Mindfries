package support

import (
	"sort"
	"strings"
	"unicode"
)

// Suggestion is a knowledge-base article proposed for a ticket, with the score
// that ranked it (so the UI and evaluators can see why).
type Suggestion struct {
	Article KBArticle `json:"article"`
	Score   int       `json:"score"`
}

// Suggest returns the top-k knowledge-base articles for a ticket, ranked by
// keyword overlap. This is the retrieval+ranking half of the "suggested reply"
// feature: in production an LLM drafts a reply from these articles, but the
// retrieval is deterministic, local, and testable — a tag match counts double
// a body-word match, so well-tagged articles surface first.
func (s *Service) Suggest(ticketID string, k int) ([]Suggestion, error) {
	t, err := s.store.GetTicket(ticketID)
	if err != nil {
		return nil, err
	}
	query := t.Subject + " " + strings.Join(messageBodies(t), " ")
	qTokens := tokenize(query)

	var ranked []Suggestion
	for _, a := range s.store.KBArticles() {
		score := articleScore(a, qTokens)
		if score > 0 {
			ranked = append(ranked, Suggestion{Article: a, Score: score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].Article.ID < ranked[j].Article.ID
	})
	if k > 0 && len(ranked) > k {
		ranked = ranked[:k]
	}
	return ranked, nil
}

func articleScore(a KBArticle, q map[string]bool) int {
	score := 0
	for _, tag := range a.Tags {
		if q[strings.ToLower(tag)] {
			score += 2 // a tag match is a strong signal
		}
	}
	for tok := range tokenize(a.Title + " " + a.Body) {
		if q[tok] {
			score++
		}
	}
	return score
}

func messageBodies(t Ticket) []string {
	out := make([]string, 0, len(t.Messages))
	for _, m := range t.Messages {
		out = append(out, m.Body)
	}
	return out
}

// tokenize lowercases, splits on non-alphanumerics, and drops very short and
// stop words so ranking keys on meaningful terms.
func tokenize(s string) map[string]bool {
	toks := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len(f) < 3 || stopWords[f] {
			continue
		}
		toks[f] = true
	}
	return toks
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "you": true,
	"with": true, "this": true, "that": true, "have": true, "not": true,
	"can": true, "but": true, "from": true, "was": true, "how": true,
}
