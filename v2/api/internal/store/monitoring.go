package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/monitor"
)

// The monitoring panel: what it reported, the evidence it uploaded, and the
// gaps when a share stopped.

// Gap is a stretch where a required share (or the panel) was off.
type Gap struct {
	ID        int64      `json:"id"`
	SessionID string     `json:"session_id"`
	Source    string     `json:"source"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Reason    string     `json:"reason"`
}

// Evidence is one stored still, screenshot or clip. The bytes are in blob
// storage at Path.
type Evidence struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Kind        string    `json:"kind"`
	TakenAt     time.Time `json:"taken_at"`
	Path        string    `json:"-"`
	Bytes       int64     `json:"bytes"`
	SHA256      string    `json:"sha256"`
	ContentType string    `json:"content_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// ReportMonitoring stores the panel's report and returns the previous one.
func (d *DB) ReportMonitoring(ctx context.Context, s monitor.State) (prev monitor.State, had bool, err error) {
	prev, had, err = d.MonitoringState(ctx, s.SessionID)
	if err != nil {
		return monitor.State{}, false, err
	}
	_, err = d.pool.Exec(ctx,
		`insert into v2.monitoring_state (session_id, panel_open, camera, screen, audio, reported_at)
		 values ($1, true, $2, $3, $4, $5)
		 on conflict (session_id) do update set panel_open = true, camera = excluded.camera,
		   screen = excluded.screen, audio = excluded.audio, reported_at = excluded.reported_at`,
		s.SessionID, s.Camera, s.Screen, s.Audio, s.ReportedAt)
	return prev, had, err
}

func (d *DB) MonitoringState(ctx context.Context, sessionID string) (monitor.State, bool, error) {
	s := monitor.State{SessionID: sessionID}
	err := d.pool.QueryRow(ctx,
		`select camera, screen, audio, reported_at from v2.monitoring_state where session_id = $1`, sessionID,
	).Scan(&s.Camera, &s.Screen, &s.Audio, &s.ReportedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return monitor.State{SessionID: sessionID}, false, nil
	}
	return s, err == nil, err
}

// OpenGap starts a gap unless one is already open for the source; it reports
// whether it started one.
func (d *DB) OpenGap(ctx context.Context, sessionID, source string, at time.Time, reason string) (bool, error) {
	ct, err := d.pool.Exec(ctx,
		`insert into v2.monitoring_gaps (session_id, source, started_at, reason) values ($1, $2, $3, $4)
		 on conflict (session_id, source) where ended_at is null do nothing`, sessionID, source, at, reason)
	return ct.RowsAffected() == 1, err
}

// CloseGap ends the open gap for a source, returning it (false if none).
func (d *DB) CloseGap(ctx context.Context, sessionID, source string, at time.Time) (Gap, bool, error) {
	var g Gap
	err := d.pool.QueryRow(ctx,
		`update v2.monitoring_gaps set ended_at = $3 where session_id = $1 and source = $2 and ended_at is null
		 returning id, session_id, source, started_at, ended_at, reason`, sessionID, source, at,
	).Scan(&g.ID, &g.SessionID, &g.Source, &g.StartedAt, &g.EndedAt, &g.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return Gap{}, false, nil
	}
	return g, err == nil, err
}

func (d *DB) Gaps(ctx context.Context, sessionID string) ([]Gap, error) {
	rows, err := d.pool.Query(ctx,
		`select id, session_id, source, started_at, ended_at, reason from v2.monitoring_gaps
		  where session_id = $1 order by started_at, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Gap{}
	for rows.Next() {
		var g Gap
		if err := rows.Scan(&g.ID, &g.SessionID, &g.Source, &g.StartedAt, &g.EndedAt, &g.Reason); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (d *DB) SaveEvidence(ctx context.Context, e Evidence) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.evidence_objects (id, session_id, kind, taken_at, storage_path, bytes, sha256, content_type, expires_at)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.ID, e.SessionID, e.Kind, e.TakenAt, e.Path, e.Bytes, e.SHA256, e.ContentType, e.ExpiresAt)
	return err
}

// LastEvidenceAt is when the session last had evidence of a kind stored.
func (d *DB) LastEvidenceAt(ctx context.Context, sessionID, kind string) (time.Time, bool, error) {
	var t *time.Time
	err := d.pool.QueryRow(ctx,
		`select max(taken_at) from v2.evidence_objects where session_id = $1 and kind = $2 and deleted_at is null`,
		sessionID, kind).Scan(&t)
	if err != nil || t == nil {
		return time.Time{}, false, err
	}
	return *t, true, nil
}

// EvidenceFor lists a session's evidence, newest last (kind "" for all).
func (d *DB) EvidenceFor(ctx context.Context, sessionID, kind string, limit int) ([]Evidence, error) {
	rows, err := d.pool.Query(ctx,
		`select id, session_id, kind, taken_at, storage_path, bytes, sha256, content_type, coalesce(expires_at, 'infinity')
		   from v2.evidence_objects
		  where session_id = $1 and deleted_at is null and ($2 = '' or kind = $2)
		  order by taken_at, id limit $3`, sessionID, kind, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Evidence{}
	for rows.Next() {
		var e Evidence
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Kind, &e.TakenAt, &e.Path, &e.Bytes, &e.SHA256, &e.ContentType, &e.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExpiredEvidence is evidence past its retention that hasn't been deleted.
func (d *DB) ExpiredEvidence(ctx context.Context, now time.Time, limit int) ([]Evidence, error) {
	rows, err := d.pool.Query(ctx,
		`select id, session_id, kind, taken_at, storage_path, bytes, sha256, content_type, expires_at
		   from v2.evidence_objects where deleted_at is null and expires_at is not null and expires_at < $1
		  order by expires_at limit $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Evidence{}
	for rows.Next() {
		var e Evidence
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Kind, &e.TakenAt, &e.Path, &e.Bytes, &e.SHA256, &e.ContentType, &e.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MarkEvidenceDeleted records that the bytes are gone.
func (d *DB) MarkEvidenceDeleted(ctx context.Context, ids []string, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := d.pool.Exec(ctx, `update v2.evidence_objects set deleted_at = $2 where id = any($1)`, ids, at)
	return err
}
