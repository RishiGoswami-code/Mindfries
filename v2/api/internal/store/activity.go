package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/activity"
)

// The tracker's rows, in v2.activity.

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// StoredEvent is one of the CLI's watcher events as stored.
type StoredEvent struct {
	Seq  int64
	At   time.Time
	Type string
	Data json.RawMessage
}

// AddActivity stores an item. An item with a DedupeKey that's already stored
// for the session is skipped (added = false), so an event sent twice by a
// restarted watcher shows once. On success it.ID is set.
func (d *DB) AddActivity(ctx context.Context, sessionID string, it activity.Item) (activity.Item, bool, error) {
	data := string(it.Data)
	if data == "" {
		data = "{}"
	}
	err := d.pool.QueryRow(ctx,
		`insert into v2.activity (session_id, at, severity, source, category, kind, text, event_seq, evidence_id, data, dedupe_key)
		 values ($1, $2, $3, $4, nullif($5, ''), $6, $7, $8, nullif($9, ''), $10::jsonb, nullif($11, ''))
		 on conflict (session_id, dedupe_key) where dedupe_key is not null do nothing
		 returning id`,
		sessionID, it.At, it.Severity, it.Source, it.Category, it.Kind, it.Text, it.EventSeq, it.EvidenceID, data, it.DedupeKey,
	).Scan(&it.ID)
	if err != nil {
		if isNoRows(err) {
			return it, false, nil
		}
		return it, false, err
	}
	return it, true, nil
}

// ActivityAfter lists a session's items with id > after, oldest first.
func (d *DB) ActivityAfter(ctx context.Context, sessionID string, after int64, limit int) ([]activity.Item, error) {
	rows, err := d.pool.Query(ctx,
		`select id, at, severity, source, coalesce(category, ''), kind, text, event_seq, coalesce(evidence_id, ''), data
		   from v2.activity where session_id = $1 and id > $2 order by id limit $3`, sessionID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []activity.Item{}
	for rows.Next() {
		var it activity.Item
		var data []byte
		if err := rows.Scan(&it.ID, &it.At, &it.Severity, &it.Source, &it.Category, &it.Kind, &it.Text, &it.EventSeq, &it.EvidenceID, &data); err != nil {
			return nil, err
		}
		if len(data) > 0 && string(data) != "{}" {
			it.Data = json.RawMessage(data)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ActivityCounts is how many items of each severity a session has.
func (d *DB) ActivityCounts(ctx context.Context, sessionID string) (map[string]int, error) {
	rows, err := d.pool.Query(ctx, `select severity, count(*) from v2.activity where session_id = $1 group by severity`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var sev string
		var n int
		if err := rows.Scan(&sev, &n); err != nil {
			return nil, err
		}
		out[sev] = n
	}
	return out, rows.Err()
}

// EventsAfter reads back the CLI's stored events, for rules that need
// context (what happened just before).
func (d *DB) EventsAfter(ctx context.Context, sessionID string, afterSeq int64, limit int) ([]StoredEvent, error) {
	rows, err := d.pool.Query(ctx,
		`select seq, occurred_at, type, data from v2.events where session_id = $1 and seq > $2 order by seq limit $3`,
		sessionID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StoredEvent{}
	for rows.Next() {
		var e StoredEvent
		var data []byte
		if err := rows.Scan(&e.Seq, &e.At, &e.Type, &data); err != nil {
			return nil, err
		}
		e.Data = json.RawMessage(data)
		out = append(out, e)
	}
	return out, rows.Err()
}
