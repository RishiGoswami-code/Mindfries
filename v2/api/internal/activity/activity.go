// Package activity is the tracker: every notification, flag and activity the
// candidate's panel shows and the hiring team reads. Items come from the CLI's
// events, the panel's own reports and the server; each is a line of what was
// recorded, never a verdict (SPEC section 16).
package activity

import (
	"encoding/json"
	"sync"
	"time"
)

// Severities.
const (
	Info   = "info"
	Notice = "notice"
	Flag   = "flag"
)

// Sources.
const (
	Watcher = "watcher" // the CLI on the candidate's computer
	Panel   = "panel"   // the monitoring panel in the browser
	Server  = "server"  // this API
)

// Categories (SPEC section 16): what kind of claim an item makes.
const (
	Observed = "observed" // we saw it happen
	Verified = "verified" // we checked it
	Signal   = "signal"   // a hint worth a look, not proof
)

// Item is one line of the tracker. Its JSON is what the panel's feed sends.
type Item struct {
	ID         int64           `json:"id"`
	At         time.Time       `json:"at"`
	Severity   string          `json:"severity"`
	Source     string          `json:"source"`
	Category   string          `json:"category,omitempty"`
	Kind       string          `json:"kind,omitempty"` // the rule or event that made it
	Text       string          `json:"text"`
	EventSeq   *int64          `json:"event_seq,omitempty"`
	EvidenceID string          `json:"evidence_id,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	DedupeKey  string          `json:"-"` // makes re-sent events not double up
}

// Valid reports whether the item's enumerations are legal.
func (i Item) Valid() bool {
	okSev := i.Severity == Info || i.Severity == Notice || i.Severity == Flag
	okSrc := i.Source == Watcher || i.Source == Panel || i.Source == Server
	okCat := i.Category == "" || i.Category == Observed || i.Category == Verified || i.Category == Signal
	return okSev && okSrc && okCat && i.Text != ""
}

// Hub fans new items out to the sessions' open feeds. It lives in one
// process; a feed that's dropped reconnects with Last-Event-ID and catches up
// from the database, so nothing depends on the hub being complete.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan Item]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[string]map[chan Item]struct{}{}} }

// Subscribe returns a channel of the session's new items and a cancel. A
// subscriber that can't keep up is closed (and reconnects).
func (h *Hub) Subscribe(sessionID string) (<-chan Item, func()) {
	ch := make(chan Item, 64)
	h.mu.Lock()
	if h.subs[sessionID] == nil {
		h.subs[sessionID] = map[chan Item]struct{}{}
	}
	h.subs[sessionID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[sessionID][ch]; ok {
			delete(h.subs[sessionID], ch)
			close(ch)
		}
		if len(h.subs[sessionID]) == 0 {
			delete(h.subs, sessionID)
		}
	}
}

// Publish sends it to the session's subscribers without blocking.
func (h *Hub) Publish(sessionID string, it Item) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[sessionID] {
		select {
		case ch <- it:
		default:
			delete(h.subs[sessionID], ch)
			close(ch)
		}
	}
}

// Subscribers is how many feeds a session has open.
func (h *Hub) Subscribers(sessionID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[sessionID])
}
