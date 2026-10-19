package watcher

import (
	"github.com/mindfries/v2/cli/internal/ledger"
)

// Where a change came from (v2/PLAN.md task 1.17, signal 5 of task 1.8).
const (
	OriginAssistant = "assistant" // an edit mindfries agent applied
	OriginCandidate = "candidate" // built up gradually
	OriginUnknown   = "unknown"   // a large block from neither
)

// LargeBlock is the number of lines arriving in one scan, not from the
// assistant, that marks a change as from neither source. Typing between
// two one-second scans stays well under it.
const LargeBlock = 30

// attribute sorts a change. Deletions and changes that add nothing aren't
// attributed.
func (w *Watcher) attribute(c Change) (origin, editID string) {
	if c.Kind == "deleted" || c.LinesAdded == 0 {
		return "", ""
	}
	if id := w.matchLedger(c); id != "" {
		return OriginAssistant, id
	}
	if c.LinesAdded >= LargeBlock {
		return OriginUnknown, ""
	}
	return OriginCandidate, ""
}

// matchLedger finds an unclaimed assistant edit that leaves the file
// exactly as it is now.
func (w *Watcher) matchLedger(c Change) string {
	edits, err := ledger.Load(w.cfg.StateDir)
	if err != nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.claimed == nil {
		w.claimed = map[string]bool{}
	}
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		if e.Path == c.Path && e.AfterSHA256 == c.SHA256 && !w.claimed[e.EditID] {
			w.claimed[e.EditID] = true
			return e.EditID
		}
	}
	return ""
}
