package watcher

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mindfries/v2/cli/internal/state"
)

// Status is what the running watcher publishes for `mindfries status` and
// for spotting a crash.
type Status struct {
	PID          int        `json:"pid"`
	StartedAt    time.Time  `json:"started_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	OfflineSince *time.Time `json:"offline_since,omitempty"`
	QueueDepth   int        `json:"queue_depth"`
	AckedSeq     int64      `json:"acked_seq"`
	// Stopped is set on a clean exit. A file without it, no longer being
	// updated, means the watcher crashed.
	Stopped    bool   `json:"stopped,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

// StaleAfter is how long without an update before a watcher counts as gone.
const StaleAfter = 5 * time.Second

func statusPath(dir string) string { return filepath.Join(dir, "watcher.json") }
func lockPath(dir string) string   { return filepath.Join(dir, "watcher.lock") }
func stopPath(dir string) string   { return filepath.Join(dir, "stop-request") }

// ReadStatus reads the watcher's status, if it has ever run.
func ReadStatus(dir string) (*Status, error) {
	b, err := os.ReadFile(statusPath(dir))
	if err != nil {
		return nil, err
	}
	var s Status
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func writeStatus(dir string, s *Status) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	return state.WriteFileAtomic(statusPath(dir), b, 0o600)
}

// Running reports whether a watcher is running for the session in dir.
func Running(dir string) bool {
	s, err := ReadStatus(dir)
	return err == nil && !s.Stopped && time.Since(s.UpdatedAt) < StaleAfter
}

// ErrAlreadyRunning means another watcher holds the lock.
var ErrAlreadyRunning = errors.New("the session watcher is already running in another terminal")

// acquire takes the watcher lock, taking over a lock left by a watcher
// that is no longer updating its status.
func acquire(dir string) error {
	for i := 0; i < 2; i++ {
		f, err := os.OpenFile(lockPath(dir), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
			return f.Close()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if Running(dir) {
			return ErrAlreadyRunning
		}
		if s, err := ReadStatus(dir); err == nil && !s.Stopped && time.Since(s.UpdatedAt) < StaleAfter {
			return ErrAlreadyRunning
		}
		// Left behind by a watcher that died.
		_ = os.Remove(lockPath(dir))
	}
	return ErrAlreadyRunning
}

func release(dir string) { _ = os.Remove(lockPath(dir)) }

// RequestStop asks a running watcher to stop for reason ("stop" or
// "submitted").
func RequestStop(dir, reason string) error {
	return state.WriteFileAtomic(stopPath(dir), []byte(reason), 0o600)
}

// WaitStopped waits for a running watcher to exit.
func WaitStopped(dir string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(lockPath(dir)); errors.Is(err, os.ErrNotExist) {
			return true
		}
		if !Running(dir) {
			if _, err := os.Stat(lockPath(dir)); errors.Is(err, os.ErrNotExist) {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func takeStopRequest(dir string) (string, bool) {
	b, err := os.ReadFile(stopPath(dir))
	if err != nil {
		return "", false
	}
	_ = os.Remove(stopPath(dir))
	r := string(b)
	if r == "" {
		r = ReasonStop
	}
	return r, true
}
