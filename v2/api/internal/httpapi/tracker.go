package httpapi

import (
	"context"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
)

// ActivityStore is the tracker's storage (implemented by store.DB).
type ActivityStore interface {
	AddActivity(ctx context.Context, sessionID string, it activity.Item) (activity.Item, bool, error)
	ActivityAfter(ctx context.Context, sessionID string, after int64, limit int) ([]activity.Item, error)
	ActivityCounts(ctx context.Context, sessionID string) (map[string]int, error)
}

// track records an item in the tracker and pushes it to the session's open
// feeds. A failure to record is logged, never the caller's problem: the
// tracker must not break the thing it is watching.
func (s *Server) track(ctx context.Context, sessionID string, it activity.Item) {
	if s.acts == nil {
		return
	}
	if it.At.IsZero() {
		it.At = s.now()
	}
	if !it.Valid() {
		s.log.Error("tracker: invalid item", "kind", it.Kind, "text", it.Text)
		return
	}
	// The request may be over by the time we write (a deferred track); keep
	// the write alive briefly on its own.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	stored, added, err := s.acts.AddActivity(wctx, sessionID, it)
	if err != nil {
		s.log.Error("tracker: store failed", "session", sessionID, "kind", it.Kind, "err", err)
		return
	}
	if added {
		s.hub.Publish(sessionID, stored)
	}
}
