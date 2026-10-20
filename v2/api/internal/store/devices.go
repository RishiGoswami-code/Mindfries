package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/session"
)

// Device codes: the browser sign-in (RFC 8628), in v2.device_codes.

var ErrDeviceNotFound = errors.New("device code not found")

func (d *DB) CreateDevice(ctx context.Context, dev auth.Device) error {
	info, _ := json.Marshal(dev.Info)
	_, err := d.pool.Exec(ctx,
		`insert into v2.device_codes (device_code_hash, user_code, expires_at, interval_seconds, status, device_info)
		 values ($1, $2, $3, $4, 'pending', $5::jsonb)`,
		dev.CodeHash, dev.UserCode, dev.ExpiresAt, dev.Interval, string(info))
	return err
}

const deviceCols = `device_code_hash, user_code, expires_at, interval_seconds, last_polled_at, status,
	coalesce(candidate_id::text, ''), grant_data, coalesce(session_id, ''), device_info`

func scanDevice(row pgx.Row) (auth.Device, error) {
	var dev auth.Device
	var grant, info []byte
	err := row.Scan(&dev.CodeHash, &dev.UserCode, &dev.ExpiresAt, &dev.Interval, &dev.LastPolledAt,
		&dev.Status, &dev.CandidateID, &grant, &dev.SessionID, &info)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Device{}, ErrDeviceNotFound
	}
	if err != nil {
		return auth.Device{}, err
	}
	_ = json.Unmarshal(info, &dev.Info)
	if len(grant) > 0 {
		var g session.Grant
		if json.Unmarshal(grant, &g) == nil {
			dev.Grant = &g
		}
	}
	return dev, nil
}

func (d *DB) DeviceByHash(ctx context.Context, hash string) (auth.Device, error) {
	return scanDevice(d.pool.QueryRow(ctx, `select `+deviceCols+` from v2.device_codes where device_code_hash = $1`, hash))
}

func (d *DB) DeviceByUserCode(ctx context.Context, userCode string) (auth.Device, error) {
	return scanDevice(d.pool.QueryRow(ctx, `select `+deviceCols+` from v2.device_codes where user_code = $1`, userCode))
}

// TouchDevice records a poll and returns when the device was last polled
// before this one (nil for the first), so the caller can answer slow_down.
func (d *DB) TouchDevice(ctx context.Context, hash string, now time.Time) (*time.Time, error) {
	var prev *time.Time
	err := d.pool.QueryRow(ctx,
		`update v2.device_codes d set last_polled_at = $2
		   from (select device_code_hash, last_polled_at from v2.device_codes where device_code_hash = $1) o
		  where d.device_code_hash = o.device_code_hash
		returning o.last_polled_at`, hash, now).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeviceNotFound
	}
	return prev, err
}

// ApproveDevice marks a pending, unexpired sign-in approved for a candidate
// and records what it entitles the CLI to. It reports false when the code
// isn't pending (expired, already decided, unknown).
func (d *DB) ApproveDevice(ctx context.Context, userCode, candidateID string, g session.Grant, now time.Time) (bool, error) {
	raw, _ := json.Marshal(g)
	ct, err := d.pool.Exec(ctx,
		`update v2.device_codes set status = 'approved', candidate_id = nullif($2, '')::uuid,
		        grant_data = $3::jsonb, approved_at = $4
		  where user_code = $1 and status = 'pending' and expires_at > $4`,
		userCode, candidateID, string(raw), now)
	return ct.RowsAffected() == 1, err
}

// DenyDevice marks a pending sign-in refused.
func (d *DB) DenyDevice(ctx context.Context, userCode string, now time.Time) (bool, error) {
	ct, err := d.pool.Exec(ctx,
		`update v2.device_codes set status = 'denied'
		  where user_code = $1 and status = 'pending' and expires_at > $2`, userCode, now)
	return ct.RowsAffected() == 1, err
}

// UseDevice spends an approved sign-in exactly once, attaching the session
// it produced. It reports false if someone else already spent it.
func (d *DB) UseDevice(ctx context.Context, hash, sessionID string) (bool, error) {
	ct, err := d.pool.Exec(ctx,
		`update v2.device_codes set status = 'used', session_id = $2
		  where device_code_hash = $1 and status = 'approved'`, hash, sessionID)
	return ct.RowsAffected() == 1, err
}

// PurgeDevices deletes sign-ins that expired before cutoff.
func (d *DB) PurgeDevices(ctx context.Context, cutoff time.Time) (int64, error) {
	ct, err := d.pool.Exec(ctx, `delete from v2.device_codes where expires_at < $1`, cutoff)
	return ct.RowsAffected(), err
}
