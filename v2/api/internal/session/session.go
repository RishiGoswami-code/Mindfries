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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"
)

// HashToken is what is stored in place of a session token: the hex SHA-256.
// Tokens are 256 random bits, so a fast hash is enough; a database leak
// doesn't hand out working tokens.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ErrNotFound is returned when a code or session doesn't exist.
var ErrNotFound = errors.New("not found")

// ErrCodeUnusable is returned when a setup code was already used.
var ErrCodeUnusable = errors.New("setup code is used")

// ErrCodeExpired is returned when a setup code ran out of time unused.
var ErrCodeExpired = errors.New("setup code has expired")

// SetupCode is a one-time dashboard code that carries an assessment's role + JD.
type SetupCode struct {
	Code            string
	AssessmentID    string // the V1 assessments.id this invite is for ("" if unlinked)
	AssessmentTitle string
	Role            string
	JD              string
	DurationMinutes int
	ExpiresAt       time.Time
	Used            bool
	Links           Links // who and what it is for (V1 ids); empty for a bare code
}

// Links ties a session or setup code to the V1 candidate, company and
// invitation it belongs to (uuid strings, or empty). The assessment has its
// own field, AssessmentID, which the grading and review code also uses.
type Links struct {
	CandidateID  string
	CompanyID    string
	InvitationID string
}

// Session is an active assessment the CLI is working against. It pins the
// resolved task so the bundle is stable for the session's lifetime.
type Session struct {
	ID              string
	Token           string // only set when the session is created; never read back
	AssessmentID    string // the V1 assessments.id, for status write-back ("" if unlinked)
	Links           Links
	AssessmentTitle string
	Role            string
	JDHash          string
	ProductSlug     string
	BankTaskID      string
	TaskVersion     string
	Brief           string
	DurationMinutes int
	Status          string     // ready | active | submitted | expired | revoked
	StartedAt       time.Time  // the clock start — set on begin
	EndsAt          time.Time  // the clock end — set on begin
	BegunAt         *time.Time // nil until begin
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
	// ErrCodeUnusable if already used, ErrCodeExpired if it ran out of time.
	// SetupCodeLive reports whether a code exists, is unused and unexpired,
	// without using it (the install script checks before serving itself).
	SetupCodeLive(ctx context.Context, code string, now time.Time) (bool, error)
	TakeSetupCode(ctx context.Context, code string, now time.Time) (SetupCode, error)
	CreateSession(ctx context.Context, s Session) error
	// SessionByToken looks a session up by its bearer token (stored hashed).
	SessionByToken(ctx context.Context, token string) (Session, error)
	// SessionByID looks a session up by id.
	SessionByID(ctx context.Context, id string) (Session, error)
	RecentSessions(ctx context.Context, limit int) ([]Session, error)
	Events(ctx context.Context, sessionID string) ([]Event, error)
	// SaveGrade upserts the grade for a submission.
	SaveGrade(ctx context.Context, g Grade) error
	// GradeBySubmission returns the submission's grade, or ErrNotFound.
	GradeBySubmission(ctx context.Context, submissionID string) (Grade, error)
	// MarkAssessmentSubmitted flips the linked V1 assessments row to 'submitted'
	// so the candidate dashboard and admin reflect that the work is handed in and
	// stop offering another setup. No-op when assessmentID is "".
	MarkAssessmentSubmitted(ctx context.Context, assessmentID string) error
	SetStatus(ctx context.Context, id, status string) error
	// LiveSessionFor is the candidate's unfinished (ready or active) session for
	// an assessment, if any: a new setup code resumes it instead of making a
	// second one.
	LiveSessionFor(ctx context.Context, candidateID, assessmentID string) (Session, bool, error)
	// RotateToken gives a session a new bearer token (the old one stops working).
	RotateToken(ctx context.Context, sessionID, newToken string) error
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
	if c.Used {
		return SetupCode{}, ErrCodeUnusable
	}
	if now.After(c.ExpiresAt) {
		return SetupCode{}, ErrCodeExpired
	}
	c.Used = true
	m.codes[code] = c
	return c, nil
}

func (m *Mem) CreateSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := HashToken(s.Token)
	s.Token = ""
	m.byToken[h] = s
	m.tokenFor[s.ID] = h
	return nil
}

func (m *Mem) SessionByID(_ context.Context, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.tokenFor[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return m.byToken[h], nil
}

func (m *Mem) SessionByToken(_ context.Context, token string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byToken[HashToken(token)]
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
	s.BegunAt = &startedAt
	m.byToken[tok] = s
	return nil
}

// Grant is what a setup code or an approved device sign-in entitles the CLI
// to: a session for one candidate on one assessment. The portal's server
// supplies it; the API never trusts a browser for any of it.
type Grant struct {
	Links           Links  `json:"links"`
	AssessmentID    string `json:"assessment_id"`
	Role            string `json:"role"`
	JD              string `json:"jd"`
	AssessmentTitle string `json:"assessment_title"`
	DurationMinutes int    `json:"duration_minutes"`
}

// Grant is the setup code's entitlement.
func (c SetupCode) Grant() Grant {
	return Grant{Links: c.Links, AssessmentID: c.AssessmentID, Role: c.Role, JD: c.JD, AssessmentTitle: c.AssessmentTitle, DurationMinutes: c.DurationMinutes}
}

// Live reports whether a session can still be worked on.
func (s Session) Live() bool { return s.Status == "ready" || s.Status == "active" }

func (m *Mem) LiveSessionFor(_ context.Context, candidateID, assessmentID string) (Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if candidateID == "" || assessmentID == "" {
		return Session{}, false, nil
	}
	for _, s := range m.byToken {
		if s.Links.CandidateID == candidateID && s.AssessmentID == assessmentID && s.Live() {
			return s, true, nil
		}
	}
	return Session{}, false, nil
}

func (m *Mem) RotateToken(_ context.Context, sessionID, newToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.tokenFor[sessionID]
	if !ok {
		return ErrNotFound
	}
	s := m.byToken[old]
	delete(m.byToken, old)
	h := HashToken(newToken)
	m.byToken[h] = s
	m.tokenFor[sessionID] = h
	return nil
}

func (m *Mem) SetupCodeLive(_ context.Context, code string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.codes[code]
	return ok && !c.Used && !now.After(c.ExpiresAt), nil
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

// MarkAssessmentSubmitted is a no-op in memory: there is no V1 assessments table.
func (m *Mem) MarkAssessmentSubmitted(_ context.Context, _ string) error { return nil }
