// Package jobs runs the server's housekeeping: ending sessions whose time has
// run out, deleting evidence past its retention period, and clearing spent
// codes. It runs in the API process on a timer; every pass is safe to repeat
// and safe to run on two instances at once.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/mindfries/v2/api/internal/blob"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

// Store is what the jobs need (implemented by store.DB).
type Store interface {
	ExpireSessions(ctx context.Context, now time.Time) ([]session.Session, error)
	ExpireReady(ctx context.Context, olderThan time.Time) (int64, error)
	ExpiredEvidence(ctx context.Context, now time.Time, limit int) ([]store.Evidence, error)
	MarkEvidenceDeleted(ctx context.Context, ids []string, at time.Time) error
	LogDeletion(ctx context.Context, sessionID, what, reason string, count int, at time.Time) error
	PurgeStale(ctx context.Context, cutoff time.Time) (int64, error)
	BlankOldTranscripts(ctx context.Context, cutoff time.Time) (int64, error)
}

// Runner is the housekeeping.
type Runner struct {
	Store         Store
	Blobs         blob.Store // nil: media can't be deleted, and isn't marked deleted
	Log           *slog.Logger
	Now           func() time.Time
	RetentionDays int
	// OnExpire is told about each session that timed out (to note it in the
	// tracker). May be nil.
	OnExpire func(ctx context.Context, s session.Session)
}

// Report is what one pass did.
type Report struct {
	SessionsExpired   int
	ReadyAbandoned    int64
	MediaDeleted      int
	TranscriptsBlank  int64
	StaleRecordsPurge int64
}

const (
	batch         = 200
	abandonReady  = 24 * time.Hour
	staleKeepDays = 7
)

func (r Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now().UTC()
}

func (r Runner) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

// Once runs every job once.
func (r Runner) Once(ctx context.Context) (Report, error) {
	var rep Report
	now := r.now()
	days := r.RetentionDays
	if days <= 0 {
		days = 90
	}

	expired, err := r.Store.ExpireSessions(ctx, now)
	if err != nil {
		return rep, err
	}
	rep.SessionsExpired = len(expired)
	for _, s := range expired {
		if r.OnExpire != nil {
			r.OnExpire(ctx, s)
		}
	}
	if rep.ReadyAbandoned, err = r.Store.ExpireReady(ctx, now.Add(-abandonReady)); err != nil {
		return rep, err
	}

	// Evidence media past its retention: delete the bytes first, then mark them
	// gone, so a failed delete is retried next pass and never leaves bytes
	// behind with a row saying they're gone.
	for {
		old, err := r.Store.ExpiredEvidence(ctx, now, batch)
		if err != nil {
			return rep, err
		}
		if len(old) == 0 || r.Blobs == nil {
			break
		}
		paths := make([]string, len(old))
		ids := make([]string, len(old))
		bySession := map[string]int{}
		for i, e := range old {
			paths[i], ids[i] = e.Path, e.ID
			bySession[e.SessionID]++
		}
		if err := r.Blobs.Delete(ctx, paths); err != nil {
			r.log().Error("retention: deleting media failed", "err", err)
			return rep, err
		}
		if err := r.Store.MarkEvidenceDeleted(ctx, ids, now); err != nil {
			return rep, err
		}
		for sid, n := range bySession {
			_ = r.Store.LogDeletion(ctx, sid, "evidence_media", "retention", n, now)
		}
		rep.MediaDeleted += len(old)
		if len(old) < batch {
			break
		}
	}

	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	if rep.TranscriptsBlank, err = r.Store.BlankOldTranscripts(ctx, cutoff); err != nil {
		return rep, err
	}
	if rep.StaleRecordsPurge, err = r.Store.PurgeStale(ctx, now.Add(-staleKeepDays*24*time.Hour)); err != nil {
		return rep, err
	}
	return rep, nil
}

// Loop runs Once every interval until ctx ends, logging what each pass did.
func (r Runner) Loop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		rep, err := r.Once(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			r.log().Error("housekeeping failed", "err", err)
		case rep != (Report{}):
			r.log().Info("housekeeping", "expired", rep.SessionsExpired, "abandoned", rep.ReadyAbandoned, "media_deleted", rep.MediaDeleted,
				"transcripts_blanked", rep.TranscriptsBlank, "stale_purged", rep.StaleRecordsPurge)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
