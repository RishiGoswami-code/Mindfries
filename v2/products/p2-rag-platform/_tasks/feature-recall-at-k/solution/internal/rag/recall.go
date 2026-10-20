package rag

// RecallAtK is the mean fraction of each case's relevant documents that appear
// among the distinct documents in the top-k retrieved chunks.
func (e *Eval) RecallAtK(cases []Case, k int) float64 {
	if len(cases) == 0 {
		return 0
	}
	var sum float64
	for _, c := range cases {
		if len(c.RelevantDocIDs) == 0 {
			continue
		}
		got := retrievedDocs(e.engine.Query(c.Query, k))
		found := 0
		for _, id := range c.RelevantDocIDs {
			if got[id] {
				found++
			}
		}
		sum += float64(found) / float64(len(c.RelevantDocIDs))
	}
	return sum / float64(len(cases))
}
