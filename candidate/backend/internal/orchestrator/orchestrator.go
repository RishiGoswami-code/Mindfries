// Package orchestrator is the Assessment Orchestrator and Evaluation Engine
// (PRD §1.2, §1.9): it turns "a candidate starts a template" into a real
// session row, turns a stream of activity events into evidence, and turns
// that evidence into a report by calling the AI Intelligence Layer
// (internal/llm) in the shape PRD §1.9 lays out — separate agents, combined
// by a Report agent at the end, not one giant model.
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/llm"
	"github.com/mindfries/candidate-backend/internal/notify"
	"github.com/mindfries/candidate-backend/internal/sandbox"
	"github.com/mindfries/candidate-backend/internal/storage"
	"github.com/mindfries/candidate-backend/internal/verify"
	"github.com/mindfries/candidate-backend/internal/ws"
)

// Orchestrator ties the data layer, the AI agents, the sandbox client and
// the real-time hub together. Every dependency is safe to hold even when
// unconfigured (sandbox and agents both report that honestly on their own);
// only DB is required.
type Orchestrator struct {
	DB      *db.DB
	Agents  *llm.Agents
	Sandbox *sandbox.Client
	Hub     *ws.Hub
	// Stills reads proctoring screenshots from Supabase Storage so the report
	// can describe what each flagged moment showed. Nil/unconfigured simply
	// omits captions; the flags still show their stills to the reviewer.
	Stills *storage.Client
	// Notifier alerts the company/admin when a candidate's task couldn't be
	// generated (a task is tried once, so a failure is final). Nil or
	// unconfigured means the failure is only recorded and logged, not pushed.
	Notifier *notify.Mailer
	// Live is the interviewer's live voice line. Nil or unconfigured means
	// interviews are held turn by turn through Agents.
	Live *llm.LiveClient
	// Verifier runs a generated task before it is given to a candidate. Nil
	// when there is nowhere to run one; the task is then kept, marked
	// "not run" — see verify.FromEnv.
	Verifier verify.Runner
	// SandboxNetwork and SandboxMaxLive are config.Config's settings of the
	// same names: what a sandbox may reach, and how many may run at once.
	SandboxNetwork string
	SandboxMaxLive int
	// RequireVerifiedEmail is config.Config's setting of the same name: an
	// invitation can only be started from an account whose address is
	// confirmed.
	RequireVerifiedEmail bool
}

// ErrEmailUnconfirmed is an invitation the candidate can't start until
// they have confirmed the address it was sent to. Its text is written for
// the candidate.
var ErrEmailUnconfirmed = errors.New("Confirm your email address before starting this assessment. The link is in the email we sent when you signed up — you can send a new one from your dashboard.")

func New(database *db.DB, agents *llm.Agents, sb *sandbox.Client, hub *ws.Hub) *Orchestrator {
	o := &Orchestrator{DB: database, Agents: agents, Sandbox: sb, Hub: hub}
	if agents != nil {
		agents.OnUsage(o.recordUsage)
	}
	return o
}

// wsEvent is the shape broadcast over a session's WebSocket room — small and
// generic on purpose, since both the candidate's own workspace and the
// admin's Global Session Monitor read the same feed for different reasons.
// ErrNotInvited and ErrTaskNotReady are returned by StartAssessment. Their
// text is shown to the candidate as it is.
var (
	ErrNotInvited   = errors.New("assessments are by invitation only, and this one isn't addressed to you")
	ErrTaskNotReady = errors.New("your assessment is still being prepared — this usually takes a minute or two. Try again shortly")
)

type wsEvent struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

func (o *Orchestrator) broadcast(sessionID, eventType string, payload any) {
	if o.Hub == nil {
		return
	}
	msg, err := json.Marshal(wsEvent{Type: eventType, Payload: payload})
	if err != nil {
		return
	}
	o.Hub.Broadcast(sessionID, msg)
}

// StartAssessment starts a session on an invitation — the only way in.
// Assessments are by invitation only (migration 0022): a company or
// Mindfries invites a candidate's email to a role, the candidate gets their
// own task for it, and nothing else can be started. The library's published
// templates are what tasks are built from, not something a candidate can
// pick.
func (o *Orchestrator) StartAssessment(ctx context.Context, candidateID, candidateEmail, candidateName, id string) (db.Session, error) {
	inv, err := o.DB.GetInvitation(ctx, id)
	if err != nil {
		return db.Session{}, ErrNotInvited
	}
	return o.startFromInvitation(ctx, candidateID, candidateEmail, candidateName, inv)
}

func (o *Orchestrator) startFromInvitation(ctx context.Context, candidateID, candidateEmail, candidateName string, inv db.Invitation) (db.Session, error) {
	if !strings.EqualFold(inv.CandidateEmail, candidateEmail) {
		// The same answer as an id that doesn't exist — confirming an
		// invitation exists but belongs to someone else is exactly the kind
		// of detail an attacker probing ids shouldn't get back.
		return db.Session{}, ErrNotInvited
	}
	if inv.Status != "invited" {
		return db.Session{}, fmt.Errorf("orchestrator: this assessment is already %s", strings.ReplaceAll(inv.Status, "_", " "))
	}
	// Its task is generated for this candidate after the invitation is made
	// (invitation_tasks.go); there is nothing to start until it exists.
	if inv.TaskStatus != "ready" || inv.TemplateID == "" {
		return db.Session{}, ErrTaskNotReady
	}
	// An invitation is addressed to an email, and an account is created by
	// typing one in. Until the address is confirmed, the person holding the
	// account hasn't shown that the invitation reached them rather than
	// someone who knew, or guessed, who was invited.
	if o.RequireVerifiedEmail {
		standing, err := o.DB.GetAccountStanding(ctx, candidateID)
		if err != nil {
			return db.Session{}, fmt.Errorf("orchestrator: checking the account: %w", err)
		}
		if !standing.EmailVerified {
			return db.Session{}, ErrEmailUnconfirmed
		}
	}

	sess, err := o.DB.StartSessionFromInvitation(ctx, candidateID, candidateName, inv)
	if err != nil {
		return db.Session{}, fmt.Errorf("orchestrator: creating session: %w", err)
	}
	if err := o.DB.SetInvitationStatus(ctx, inv.ID, "in_progress"); err != nil {
		// The session is real either way; log rather than fail a candidate's
		// entry over the invitation's own status bookkeeping.
		slog.Error("orchestrator: marking invitation in_progress failed", "invitation", inv.ID, "error", err)
	}
	// The hiring team's pipeline follows the session too (db/applications.go).
	if err := o.DB.MarkApplicationStarted(ctx, inv.ID); err != nil {
		slog.Error("orchestrator: moving the application to in_progress failed", "invitation", inv.ID, "error", err)
	}

	o.assignVariant(ctx, sess)
	sess = o.provisionAndAnnounce(ctx, sess, candidateName, candidateEmail)
	return sess, nil
}

// provisionAndAnnounce is the part both entry points share: an optional
// sandbox (never fails the session over it — "real, or an honest failure,"
// never "fail the whole flow over an optional piece that isn't wired up")
// and the real-time broadcast every session start makes.
func (o *Orchestrator) provisionAndAnnounce(ctx context.Context, sess db.Session, candidateName, candidateEmail string) db.Session {
	// The sandbox is where the candidate's workspace lives when there is one
	// (sandbox_workspace.go). A session without one — Daytona not
	// configured, or not answering — runs in the browser, as before.
	began := time.Now()
	sess, _ = o.setUpSandbox(ctx, sess, candidateName, candidateEmail)
	// Making the machine and putting the task on it takes around ten
	// seconds (longer when it fails and the session falls back to the
	// browser). That is our time, not the candidate's: the session's clock
	// starts now, when there is a workspace to open.
	if time.Since(began) > time.Second {
		if startedAt, err := o.DB.RestartClock(context.WithoutCancel(ctx), sess.ID); err != nil {
			slog.Error("orchestrator: restarting the session clock after setup failed", "session", sess.ID, "error", err)
		} else {
			sess.StartedAt = startedAt
		}
	}
	o.broadcast(sess.ID, "session_started", sess)
	return sess
}

// TeardownSandbox is teardownSandbox's exported form, for a caller that
// doesn't already have the session loaded — today, the admin reset
// endpoint, which resets a session back to a fresh state and shouldn't
// leave whatever sandbox it had running (or its ID pointing at nothing).
func (o *Orchestrator) TeardownSandbox(ctx context.Context, sessionID string) error {
	sess, err := o.DB.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("orchestrator: loading session to tear down its sandbox: %w", err)
	}
	o.teardownSandbox(ctx, sess)
	return nil
}

// teardownSandbox deletes a session's sandbox, if it has one, and clears the
// column either way — a delete failure shouldn't leave a stale ID pointing
// at a sandbox this backend will never try to clean up again. Shared by
// Submit (a session ending normally) and the admin reset support-override
// (a session being forced back to a fresh state) — the two places a
// session's sandbox actually needs to go away. Best-effort and never
// returns an error: whoever calls this has a session-lifecycle action to
// finish, and a Daytona API hiccup shouldn't block that.
func (o *Orchestrator) teardownSandbox(ctx context.Context, sess db.Session) {
	if sess.SandboxID == nil {
		return
	}
	if o.Sandbox != nil && o.Sandbox.Configured() {
		if err := o.Sandbox.DeleteSandbox(ctx, *sess.SandboxID); err != nil {
			slog.Error("orchestrator: sandbox teardown failed", "session", sess.ID, "sandbox", *sess.SandboxID, "error", err)
		}
	}
	if err := o.DB.SetSandboxID(ctx, sess.ID, nil); err != nil {
		slog.Error("orchestrator: clearing sandbox id failed", "session", sess.ID, "error", err)
	}
}

// RecordEvents stores a batch of telemetry (PRD §1.7) and fans it out live to
// anyone watching this session's room — the admin's Global Session Monitor
// in particular.
func (o *Orchestrator) RecordEvents(ctx context.Context, sessionID string, events []db.NewActivityEvent) error {
	if err := o.DB.InsertActivityEvents(ctx, sessionID, events); err != nil {
		return err
	}
	for _, e := range events {
		o.broadcast(sessionID, "activity_event", e)
	}
	return nil
}

// Submit marks a session submitted and runs evaluation synchronously. The
// HTTP handler is the one that decides whether to wait for this or return
// immediately and let it run in the background (see httpapi's submit
// handler) — this method itself just does the work, so it's usable either
// way and independently testable.
func (o *Orchestrator) Submit(ctx context.Context, sessionID string) error {
	sess, err := o.DB.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("orchestrator: loading session to submit: %w", err)
	}

	// Conditional on the session still being "live" at write time — the
	// HTTP handler already checked this on read, but only this atomic update
	// is what actually decides between two concurrent submits (see
	// db.MarkSubmitted's own comment). Losing this race is the expected,
	// safe outcome, not an error worth logging as one.
	if err := o.DB.MarkSubmitted(ctx, sessionID); err != nil {
		if errors.Is(err, db.ErrAlreadySubmitted) {
			return nil
		}
		return err
	}
	if sess.AssessmentID != nil {
		// This session came from a real invitation — its lifecycle follows
		// the session's, the same way startFromInvitation moved it to
		// in_progress. A template-only (self-serve) session has no
		// invitation to update.
		if err := o.DB.SetInvitationStatus(ctx, *sess.AssessmentID, "submitted"); err != nil {
			slog.Error("orchestrator: marking invitation submitted failed", "invitation", *sess.AssessmentID, "error", err)
		}
		// Capped at the session's own length: a submit that arrives after
		// the clock ran out (the interview ran past it, or the server
		// submitted an abandoned session hours later) isn't time spent on
		// the task.
		took := int(math.Ceil(time.Since(sess.StartedAt).Minutes()))
		if sess.DurationMin > 0 {
			took = min(took, sess.DurationMin)
		}
		if err := o.DB.MarkApplicationCompleted(ctx, *sess.AssessmentID, took); err != nil {
			slog.Error("orchestrator: marking the application completed failed", "invitation", *sess.AssessmentID, "error", err)
		}
	}
	// The candidate is done editing the moment they submit — the sandbox
	// (if one was ever provisioned) has no further reason to exist, and
	// evaluation reads recorded telemetry, not the live sandbox.
	o.teardownSandbox(ctx, sess)

	o.broadcast(sessionID, "session_submitted", map[string]string{"sessionId": sessionID})
	return o.Evaluate(ctx, sessionID)
}
