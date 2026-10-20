// Package session is the storage layer for V2 assessment sessions: the setup
// codes the dashboard mints, and the sessions the CLI exchanges them for. It is
// pure storage — the HTTP layer orchestrates the engine (resolve a task) and
// the bundle (sign it); this package just persists and looks up.
//
// Store has a Postgres implementation (in internal/store, against v2.sessions /
// v2.setup_codes) and the in-memory Mem here for tests.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a code or session doesn't exist.
var ErrNotFound = errors.New("not found")

// ErrCodeUnusable is returned when a setup code is already used or expired.
var ErrCodeUnusable = errors.New("setup code is used or expired")

// SetupCode is a one-time dashboard code that carries an assessment's role + JD.
type SetupCode struct {
	Code            string
	AssessmentTitle string
	Role            string
	JD              string
	DurationMinutes int
	ExpiresAt       time.Time
	Used            bool
}

// Session is an active assessment the CLI is working against. It pins the
// resolved task so the bundle is stable for the session's lifetime.
type Session struct {
	ID              string
	Token           string
	AssessmentTitle string
	Role            string
	JDHash          string
	ProductSlug     string
	BankTaskID      string
	TaskVersion     string
	Brief           string
	Status          string // active | submitted | expired | revoked
	StartedAt       time.Time
	EndsAt          time.Time
}

// Event is one watcher evidence event, stored as received (hash-chained).
type Event struct {
	Seq        int64           `json:"seq"`
	OccurredAt time.Time       `json:"occurred_at"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	PrevHash   string          `json:"prev_hash"`
	Hash       string          `json:"hash"`
}

// Snapshot is a periodic git-diff snapshot of the workspace.
type Snapshot struct {
	Seq         int64     `json:"seq"`
	TakenAt     time.Time `json:"taken_at"`
	BaseCommit  string    `json:"base_commit"`
	HeadCommit  string    `json:"head_commit"`
	Patch       []byte    `json:"patch"`
	PatchSHA256 string    `json:"patch_sha256"`
}

// Store persists setup codes, sessions, and session evidence.
type Store interface {
	CreateSetupCode(ctx context.Context, c SetupCode) error
	// TakeSetupCode marks the code used and returns it; ErrNotFound if missing,
	// ErrCodeUnusable if already used or expired.
	TakeSetupCode(ctx context.Context, code string, now time.Time) (SetupCode, error)
	CreateSession(ctx context.Context, s Session) error
	SessionByToken(ctx context.Context, token string) (Session, error)
	SetStatus(ctx context.Context, id, status string) error

	// AppendEvents stores events idempotently by (session, seq). It returns the
	// highest seq now stored (the CLI deletes up to it), how many were newly
	// stored, and how many were duplicates.
	AppendEvents(ctx context.Context, sessionID string, events []Event) (acked int64, stored, duplicates int, err error)
	// LastAckedSeq is the highest event seq stored for a session (0 if none).
	LastAckedSeq(ctx context.Context, sessionID string) (int64, error)
	// SaveSnapshot stores a snapshot idempotently by (session, seq).
	SaveSnapshot(ctx context.Context, sessionID string, s Snapshot) error
}

// Mem is an in-memory Store for tests.
type Mem struct {
	mu        sync.Mutex
	codes     map[string]SetupCode
	byToken   map[string]Session
	tokenFor  map[string]string          // session id -> token
	events    map[string]map[int64]Event // session id -> seq -> event
	snapshots map[string]map[int64]Snapshot
}

// NewMem returns an empty in-memory store.
func NewMem() *Mem {
	return &Mem{
		codes: map[string]SetupCode{}, byToken: map[string]Session{}, tokenFor: map[string]string{},
		events: map[string]map[int64]Event{}, snapshots: map[string]map[int64]Snapshot{},
	}
}

func (m *Mem) AppendEvents(_ context.Context, sessionID string, events []Event) (int64, int, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.events[sessionID] == nil {
		m.events[sessionID] = map[int64]Event{}
	}
	stored := 0
	for _, e := range events {
		if _, ok := m.events[sessionID][e.Seq]; ok {
			continue
		}
		m.events[sessionID][e.Seq] = e
		stored++
	}
	var acked int64
	for seq := range m.events[sessionID] {
		if seq > acked {
			acked = seq
		}
	}
	return acked, stored, len(events) - stored, nil
}

func (m *Mem) LastAckedSeq(_ context.Context, sessionID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var acked int64
	for seq := range m.events[sessionID] {
		if seq > acked {
			acked = seq
		}
	}
	return acked, nil
}

func (m *Mem) SaveSnapshot(_ context.Context, sessionID string, s Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snapshots[sessionID] == nil {
		m.snapshots[sessionID] = map[int64]Snapshot{}
	}
	if _, ok := m.snapshots[sessionID][s.Seq]; !ok {
		m.snapshots[sessionID][s.Seq] = s
	}
	return nil
}

func (m *Mem) CreateSetupCode(_ context.Context, c SetupCode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[c.Code] = c
	return nil
}

func (m *Mem) TakeSetupCode(_ context.Context, code string, now time.Time) (SetupCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.codes[code]
	if !ok {
		return SetupCode{}, ErrNotFound
	}
	if c.Used || now.After(c.ExpiresAt) {
		return SetupCode{}, ErrCodeUnusable
	}
	c.Used = true
	m.codes[code] = c
	return c, nil
}

func (m *Mem) CreateSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byToken[s.Token] = s
	m.tokenFor[s.ID] = s.Token
	return nil
}

func (m *Mem) SessionByToken(_ context.Context, token string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byToken[token]
	if !ok {
		return Session{}, ErrNotFound
	}
	return s, nil
}

func (m *Mem) SetStatus(_ context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokenFor[id]
	if !ok {
		return ErrNotFound
	}
	s := m.byToken[tok]
	s.Status = status
	m.byToken[tok] = s
	return nil
}
