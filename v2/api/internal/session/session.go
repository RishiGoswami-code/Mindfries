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
	"sort"
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
	DurationMinutes int
	Status          string    // ready | active | submitted | expired | revoked
	StartedAt       time.Time // the clock start — set on begin
	EndsAt          time.Time // the clock end — set on begin
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

// Submission is what the candidate hands in: the manifest plus (once uploaded)
// the git bundle. Status: awaiting_bundle -> verified | rejected.
type Submission struct {
	ID              string
	SessionID       string
	TaskVersion     string
	Branch          string
	BaseCommit      string
	HeadCommit      string
	CommitMessage   string
	BundleSHA256    string
	BundleBytes     int64
	TaskFilesIntact bool
	Manifest        json.RawMessage // the full submission manifest as received
	Status          string
	Bundle          []byte // the uploaded git bundle (nil until uploaded)
	CreatedAt       time.Time
	VerifiedAt      *time.Time
}

// Grade is the stored result of running a submission against its task's hidden
// acceptance tests (package grade). One per submission, produced out-of-process
// by an isolated runner.
type Grade struct {
	SubmissionID string    `json:"submission_id"`
	Passed       bool      `json:"passed"`
	Check        []string  `json:"check"`
	Output       string    `json:"output"`
	GradedAt     time.Time `json:"graded_at"`
}

// Store persists setup codes, sessions, evidence, and submissions.
type Store interface {
	CreateSetupCode(ctx context.Context, c SetupCode) error
	// TakeSetupCode marks the code used and returns it; ErrNotFound if missing,
	// ErrCodeUnusable if already used or expired.
	TakeSetupCode(ctx context.Context, code string, now time.Time) (SetupCode, error)
	CreateSession(ctx context.Context, s Session) error
	SessionByToken(ctx context.Context, token string) (Session, error)
	SetStatus(ctx context.Context, id, status string) error
	// Begin starts the clock: status -> active, with the given start/end times.
	Begin(ctx context.Context, id string, startedAt, endsAt time.Time) error

	// AppendEvents stores events idempotently by (session, seq). It returns the
	// highest seq now stored (the CLI deletes up to it), how many were newly
	// stored, and how many were duplicates.
	AppendEvents(ctx context.Context, sessionID string, events []Event) (acked int64, stored, duplicates int, err error)
	// LastAckedSeq is the highest event seq stored for a session (0 if none).
	LastAckedSeq(ctx context.Context, sessionID string) (int64, error)
	// SaveSnapshot stores a snapshot idempotently by (session, seq).
	SaveSnapshot(ctx context.Context, sessionID string, s Snapshot) error

	// CreateSubmission stores a new submission (status awaiting_bundle).
	CreateSubmission(ctx context.Context, sub Submission) error
	// SubmissionBySession returns the session's submission, or ErrNotFound.
	SubmissionBySession(ctx context.Context, sessionID string) (Submission, error)
	// SubmissionByID returns a submission by id, or ErrNotFound.
	SubmissionByID(ctx context.Context, id string) (Submission, error)
	// CompleteSubmission attaches the uploaded bundle and sets the final status.
	CompleteSubmission(ctx context.Context, id string, bundle []byte, status string, verifiedAt time.Time) error

	// Read + grade methods for the admin review surface (GRADE step).
	SessionByID(ctx context.Context, id string) (Session, error)
	RecentSessions(ctx context.Context, limit int) ([]Session, error)
	Events(ctx context.Context, sessionID string) ([]Event, error)
	// SaveGrade upserts the grade for a submission.
	SaveGrade(ctx context.Context, g Grade) error
	// GradeBySubmission returns the submission's grade, or ErrNotFound.
	GradeBySubmission(ctx context.Context, submissionID string) (Grade, error)
}

// Mem is an in-memory Store for tests.
type Mem struct {
	mu        sync.Mutex
	codes     map[string]SetupCode
	byToken   map[string]Session
	tokenFor  map[string]string          // session id -> token
	events    map[string]map[int64]Event // session id -> seq -> event
	snapshots map[string]map[int64]Snapshot
	subs      map[string]Submission // id -> submission
	subBySess map[string]string     // session id -> submission id
	grades    map[string]Grade      // submission id -> grade
}

// NewMem returns an empty in-memory store.
func NewMem() *Mem {
	return &Mem{
		codes: map[string]SetupCode{}, byToken: map[string]Session{}, tokenFor: map[string]string{},
		events: map[string]map[int64]Event{}, snapshots: map[string]map[int64]Snapshot{},
		subs: map[string]Submission{}, subBySess: map[string]string{}, grades: map[string]Grade{},
	}
}

func (m *Mem) SessionByID(_ context.Context, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tok, ok := m.tokenFor[id]; ok {
		return m.byToken[tok], nil
	}
	return Session{}, ErrNotFound
}

func (m *Mem) RecentSessions(_ context.Context, limit int) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Session
	for _, tok := range m.tokenFor {
		out = append(out, m.byToken[tok])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Mem) Events(_ context.Context, sessionID string) ([]Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Event
	for _, e := range m.events[sessionID] {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
}

func (m *Mem) SaveGrade(_ context.Context, g Grade) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.grades[g.SubmissionID] = g
	return nil
}

func (m *Mem) GradeBySubmission(_ context.Context, submissionID string) (Grade, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.grades[submissionID]; ok {
		return g, nil
	}
	return Grade{}, ErrNotFound
}

func (m *Mem) CreateSubmission(_ context.Context, sub Submission) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[sub.ID] = sub
	m.subBySess[sub.SessionID] = sub.ID
	return nil
}

func (m *Mem) SubmissionBySession(_ context.Context, sessionID string) (Submission, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.subBySess[sessionID]
	if !ok {
		return Submission{}, ErrNotFound
	}
	return m.subs[id], nil
}

func (m *Mem) SubmissionByID(_ context.Context, id string) (Submission, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subs[id]
	if !ok {
		return Submission{}, ErrNotFound
	}
	return s, nil
}

func (m *Mem) CompleteSubmission(_ context.Context, id string, bundle []byte, status string, verifiedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subs[id]
	if !ok {
		return ErrNotFound
	}
	s.Bundle = bundle
	s.Status = status
	s.VerifiedAt = &verifiedAt
	m.subs[id] = s
	return nil
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

func (m *Mem) Begin(_ context.Context, id string, startedAt, endsAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokenFor[id]
	if !ok {
		return ErrNotFound
	}
	s := m.byToken[tok]
	s.Status, s.StartedAt, s.EndsAt = "active", startedAt, endsAt
	m.byToken[tok] = s
	return nil
}
