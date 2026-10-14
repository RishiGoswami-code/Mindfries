package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/sandbox"
	"github.com/mindfries/candidate-backend/internal/workspace"
)

// A session's workspace in its sandbox.
//
// When Daytona is configured, a session gets a real machine: the task is
// put there when the session starts, the IDE's terminal is a shell on it,
// and the files the editor shows are the files on its disk. The candidate
// can install and run whatever the task needs.
//
// When Daytona isn't configured, or the sandbox couldn't be made ready, the
// session works as it did before — entirely in the browser — and nothing
// here applies. Which of the two a session is on is recorded once, at the
// start, as a sandbox_ready event; everything asks SandboxReady.

const (
	// eventSandboxReady marks a session whose sandbox holds its task.
	eventSandboxReady = "sandbox_ready"
	// eventTerminalCommand is a command the candidate ran, with how it
	// ended. The in-browser workspace reports the same event from the page;
	// in a sandbox the shell itself reports it and this backend records it.
	eventTerminalCommand = "terminal_command"
	// eventCommandRecordAltered is something that makes the command record
	// incomplete or untrustworthy: the hooks that report commands were
	// changed, or their marker was printed by hand (workspace/integrity.go).
	// The report says so.
	eventCommandRecordAltered = "command_record_altered"

	// A sandbox is stopped by Daytona after this long with nothing
	// happening in it, and started again on the next use.
	sandboxAutoStopMin = 30
	// …and deleted by Daytona this long after stopping, whatever this
	// backend does. Sessions are hours at most; this is the backstop for a
	// sandbox whose session ended in a way that skipped teardown.
	sandboxAutoDeleteMin = 24 * 60
	// How long a session's start will wait for its sandbox to be ready
	// before going ahead in the browser instead.
	sandboxSetupTimeout = 75 * time.Second
)

// ErrNoSandbox means the session's workspace is in the browser.
var ErrNoSandbox = errors.New("orchestrator: this session has no sandbox workspace")

// ErrWorkFrozen means the interview has begun and the work can't change.
var ErrWorkFrozen = errors.New("orchestrator: the interview has begun; the work can no longer change")

// setUpSandbox creates the session's sandbox and puts the task in it. It
// reports whether the session ended up with a ready sandbox; when it
// didn't, nothing is left behind and the session carries on in the browser.
func (o *Orchestrator) setUpSandbox(ctx context.Context, sess db.Session, candidateName, candidateEmail string) (db.Session, bool) {
	if o.Sandbox == nil || !o.Sandbox.Configured() {
		return sess, false
	}
	// Each sandbox is a running machine on one Daytona account. At the
	// ceiling a new session starts in the browser — a working workspace —
	// rather than failing or queueing behind an unknown wait.
	if o.SandboxMaxLive > 0 {
		if n, err := o.DB.CountLiveSandboxes(ctx); err == nil && n >= o.SandboxMaxLive {
			slog.Warn("orchestrator: sandbox ceiling reached; the session will run in the browser", "session", sess.ID, "live", n, "ceiling", o.SandboxMaxLive)
			return sess, false
		}
	}
	ctx, cancel := context.WithTimeout(ctx, sandboxSetupTimeout)
	defer cancel()
	started := time.Now()

	fail := func(step string, err error, sandboxID string) (db.Session, bool) {
		slog.Error("orchestrator: sandbox setup failed; the session will run in the browser", "session", sess.ID, "step", step, "error", err)
		if sandboxID != "" {
			// Not with ctx: it may be the reason we are here.
			if derr := o.Sandbox.DeleteSandbox(context.WithoutCancel(ctx), sandboxID); derr != nil {
				slog.Error("orchestrator: deleting a half-made sandbox failed", "session", sess.ID, "sandbox", sandboxID, "error", derr)
			}
			_ = o.DB.SetSandboxID(context.WithoutCancel(ctx), sess.ID, nil)
		}
		degraded := "degraded"
		_ = o.DB.UpdateSessionState(context.WithoutCancel(ctx), sess.ID, db.SessionStatePatch{SandboxHealth: &degraded})
		sess.SandboxHealth, sess.SandboxID = degraded, nil
		return sess, false
	}

	opts := sandbox.CreateOptions{
		AutoStopInterval:   sandboxAutoStopMin,
		AutoDeleteInterval: sandboxAutoDeleteMin,
		Labels:             map[string]string{"mindfries-session": sess.ID},
	}
	switch o.SandboxNetwork {
	case "none":
		opts.NetworkBlockAll = true
	case "essentials":
		// Setting any list is what turns the restriction on; Daytona's
		// essential services stay reachable alongside it. The one address
		// listed is reserved for documentation and routes nowhere.
		opts.NetworkAllowList = "192.0.2.1/32"
	}
	sb, err := o.Sandbox.CreateSandbox(ctx, opts)
	if err != nil {
		return fail("create", err, "")
	}
	// Recorded before anything else can fail, so a sandbox is never running
	// with nothing pointing at it (teardownSandbox deletes by this id).
	if err := o.DB.SetSandboxID(ctx, sess.ID, &sb.ID); err != nil {
		return fail("record id", err, sb.ID)
	}
	sess.SandboxID = &sb.ID
	if err := o.Sandbox.EnsureStarted(ctx, sb.ID); err != nil {
		return fail("start", err, sb.ID)
	}

	content := o.TemplateContent(ctx, sess)
	w := workspace.Workspace{Client: o.Sandbox, SandboxID: sb.ID}
	if err := w.Seed(ctx, content.StarterFiles, candidateName, candidateEmail); err != nil {
		return fail("seed", err, sb.ID)
	}

	payload, _ := json.Marshal(map[string]any{"files": len(content.StarterFiles), "setupMs": time.Since(started).Milliseconds(), "network": o.NetworkPolicy()})
	if err := o.DB.InsertActivityEvents(ctx, sess.ID, []db.NewActivityEvent{{EventType: eventSandboxReady, Payload: payload}}); err != nil {
		return fail("record ready", err, sb.ID)
	}
	slog.Info("orchestrator: sandbox ready", "session", sess.ID, "sandbox", sb.ID, "took", time.Since(started).Round(time.Millisecond))
	return sess, true
}

// NetworkPolicy is what a sandbox may reach: "open", "essentials" or "none".
func (o *Orchestrator) NetworkPolicy() string {
	if o.SandboxNetwork == "none" || o.SandboxNetwork == "essentials" {
		return o.SandboxNetwork
	}
	return "open"
}

// SandboxReady reports whether the session's workspace is in a sandbox.
func (o *Orchestrator) SandboxReady(ctx context.Context, sess db.Session) bool {
	if sess.SandboxID == nil || o.Sandbox == nil || !o.Sandbox.Configured() {
		return false
	}
	ready, err := o.DB.HasEvent(ctx, sess.ID, eventSandboxReady)
	return err == nil && ready
}

// SandboxWorkspace returns the session's project in its sandbox, started
// and ready to use. ErrNoSandbox when the session is a browser one.
func (o *Orchestrator) SandboxWorkspace(ctx context.Context, sess db.Session) (workspace.Workspace, error) {
	if !o.SandboxReady(ctx, sess) {
		return workspace.Workspace{}, ErrNoSandbox
	}
	if err := o.Sandbox.EnsureStarted(ctx, *sess.SandboxID); err != nil {
		return workspace.Workspace{}, err
	}
	return workspace.Workspace{Client: o.Sandbox, SandboxID: *sess.SandboxID}, nil
}

// WorkFrozen reports whether the interview has begun — the point after
// which the work is what will be evaluated and can't be changed
// (RecordSnapshot explains why).
func (o *Orchestrator) WorkFrozen(ctx context.Context, sessionID string) bool {
	frozen, err := o.DB.HasEvent(ctx, sessionID, eventInterview)
	return err == nil && frozen
}

// CaptureWorkspace records the candidate's work as it stands. For a sandbox
// session that is read from the sandbox's own disk — the files as they
// really are, whatever the page believes — and `fromBrowser` is ignored.
// For a browser session `fromBrowser` is the work. If the sandbox can't be
// read just now, what the browser sent is used rather than nothing.
func (o *Orchestrator) CaptureWorkspace(ctx context.Context, sess db.Session, fromBrowser map[string]string, checkpoint bool) error {
	files := fromBrowser
	if w, err := o.SandboxWorkspace(ctx, sess); err == nil {
		if snap, _, serr := w.Snapshot(ctx); serr == nil {
			files = snap
			o.checkCommandRecord(ctx, sess.ID, w, snap)
		} else {
			slog.Error("orchestrator: reading the sandbox for a snapshot failed; using the page's copy", "session", sess.ID, "error", serr)
		}
	} else if !errors.Is(err, ErrNoSandbox) {
		slog.Error("orchestrator: the sandbox isn't reachable for a snapshot; using the page's copy", "session", sess.ID, "error", err)
	}
	return o.recordSnapshot(ctx, sess.ID, files, checkpoint)
}

// checkCommandRecord looks for changes that make the command record
// untrustworthy and records what it finds, once per distinct finding. A
// check that can't be run is logged, not reported: it says nothing about
// the candidate.
func (o *Orchestrator) checkCommandRecord(ctx context.Context, sessionID string, w workspace.Workspace, files map[string]string) {
	problems, err := w.CheckCommandHooks(ctx)
	if err != nil {
		slog.Error("orchestrator: checking the shell's command hooks", "session", sessionID, "error", err)
	}
	if paths := workspace.FilesMentionTheMarker(files); len(paths) > 0 {
		problems = append(problems, "project files contain the marker the hooks print, or name the hooks: "+strings.Join(paths, ", "))
	}
	for _, problem := range problems {
		o.recordAlteration(ctx, sessionID, problem)
	}
}

func (o *Orchestrator) recordAlteration(ctx context.Context, sessionID, problem string) {
	seen, err := o.DB.EventPayloadsOfType(ctx, sessionID, eventCommandRecordAltered)
	if err == nil {
		for _, raw := range seen {
			var p struct {
				Problem string `json:"problem"`
			}
			if json.Unmarshal(raw, &p) == nil && p.Problem == problem {
				return
			}
		}
	}
	payload, _ := json.Marshal(map[string]string{"problem": problem})
	if err := o.RecordEvents(ctx, sessionID, []db.NewActivityEvent{{EventType: eventCommandRecordAltered, Payload: payload}}); err != nil {
		slog.Error("orchestrator: recording a change to the command record", "session", sessionID, "error", err)
	}
}

// RecordTerminalCommand stores a command the sandbox's shell reported.
func (o *Orchestrator) RecordTerminalCommand(ctx context.Context, sessionID, command string, exitCode int, took time.Duration) {
	// A command that works on the hooks is reported by them before it runs
	// — removing the hooks from inside the shell shows up in the very record
	// it removes them from.
	if workspace.SuspiciousCommand(command) {
		shown := command
		if len(shown) > 200 {
			shown = shown[:200] + "…"
		}
		o.recordAlteration(ctx, sessionID, "a command touched the hooks that report commands, or printed their marker: "+shown)
	}
	if len(command) > 300 {
		command = command[:300]
	}
	payload, _ := json.Marshal(map[string]any{"command": command, "exitCode": exitCode, "ms": took.Milliseconds(), "source": "sandbox"})
	if err := o.RecordEvents(ctx, sessionID, []db.NewActivityEvent{{EventType: eventTerminalCommand, Payload: payload}}); err != nil {
		slog.Error("orchestrator: recording a terminal command failed", "session", sessionID, "error", err)
	}
}
