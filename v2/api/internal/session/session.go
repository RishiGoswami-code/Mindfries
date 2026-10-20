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

// Store persists setup codes and sessions.
type Store interface {
	CreateSetupCode(ctx context.Context, c SetupCode) error
	// TakeSetupCode marks the code used and returns it; ErrNotFound if missing,
	// ErrCodeUnusable if already used or expired.
	TakeSetupCode(ctx context.Context, code string, now time.Time) (SetupCode, error)
	CreateSession(ctx context.Context, s Session) error
	SessionByToken(ctx context.Context, token string) (Session, error)
	SetStatus(ctx context.Context, id, status string) error
}

// Mem is an in-memory Store for tests.
type Mem struct {
	mu       sync.Mutex
	codes    map[string]SetupCode
	byToken  map[string]Session
	tokenFor map[string]string // session id -> token
}

// NewMem returns an empty in-memory store.
func NewMem() *Mem {
	return &Mem{codes: map[string]SetupCode{}, byToken: map[string]Session{}, tokenFor: map[string]string{}}
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
