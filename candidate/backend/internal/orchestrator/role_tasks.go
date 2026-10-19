package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/llm"
)

// A job role's task is generated once, when the company creates the role or
// edits its brief, and shared by everyone invited to it (migration 0028). This
// worker is the role-level counterpart to RunInvitationTaskWorker: it polls for
// roles waiting on a task, generates one, and links it — so the task is ready
// before any invitation goes out and no candidate watches a "preparing" screen.
//
// The invitation worker still runs alongside, for invitations that have no
// role (an admin invite made without one) — their task is still generated per
// invitation, the old way.

// RunRoleTaskWorker generates tasks for roles waiting on one until ctx ends.
func (o *Orchestrator) RunRoleTaskWorker(ctx context.Context) {
	if o.Agents == nil || !o.Agents.Configured() {
		slog.Warn("orchestrator: no AI service is configured; roles will wait for their tasks")
	}
	for {
		o.settleTasks(ctx)
		wait := invitationTaskPoll
		for {
			more, pause := o.prepareNextRoleTask(ctx)
			if pause > 0 {
				wait = pause
				break
			}
			if !more || ctx.Err() != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// settleTasks is the worker's sweep for tasks no generation will ever finish:
// generations that stalled twice are marked failed (and a person alerted), and
// invitations waiting on a role whose task is ready or has failed are brought
// in line with it (db.SettleRoleInvitations). Without it a candidate could
// watch "being prepared" for ever while the worker found nothing to do.
func (o *Orchestrator) settleTasks(ctx context.Context) {
	stalled, err := o.DB.FailStalledTasks(ctx)
	if err != nil {
		slog.Error("orchestrator: failing stalled task generations", "error", err)
	}
	for _, t := range stalled {
		slog.Error("orchestrator: a task generation stalled twice; marked failed", "id", t.ID, "role", t.RoleTitle, "candidate", t.CandidateEmail)
		o.alertTaskFailure(ctx, t.ID, "", t.RoleTitle, t.CandidateEmail, "Task generation was interrupted twice before it finished (the backend restarted while it ran).")
	}
	adopted, failed, err := o.DB.SettleRoleInvitations(ctx)
	if err != nil {
		slog.Error("orchestrator: settling invitations with their role's task", "error", err)
		return
	}
	if adopted > 0 {
		slog.Info("orchestrator: invitations given their role's task", "count", adopted)
	}
	if failed > 0 {
		slog.Warn("orchestrator: invitations marked failed because their role's task failed", "count", failed)
	}
}

// prepareNextRoleTask handles one role waiting on a task. It reports whether
// there was one, and how long to pause first when the AI service's quota has
// run out (so the worker doesn't hammer a used-up quota).
func (o *Orchestrator) prepareNextRoleTask(ctx context.Context) (bool, time.Duration) {
	if o.Agents == nil || !o.Agents.Configured() {
		return false, 0
	}
	job, err := o.DB.ClaimRoleTask(ctx)
	if err != nil {
		slog.Error("orchestrator: claiming a role to make its task", "error", err)
		return false, 0
	}
	if job == nil {
		return false, 0
	}
	ctx, cancel := context.WithTimeout(ctx, invitationTaskCap)
	defer cancel()
	started := time.Now()
	err = o.makeRoleTask(ctx, *job)
	var quota *llm.QuotaError
	var busy *llm.BusyError
	if errors.As(err, &busy) {
		slog.Warn("orchestrator: the AI service is busy; the role will wait", "role", job.RoleID, "pause", busyPause, "error", busy.Message)
		reason := "Waiting: the AI service is busy right now. The task will be written shortly — no action is needed."
		if derr := o.DB.DeferRoleTask(context.WithoutCancel(ctx), *job, reason); derr != nil {
			slog.Error("orchestrator: putting a role back in the queue", "role", job.RoleID, "error", derr)
		}
		return true, busyPause
	}
	if errors.As(err, &quota) {
		pause := quota.RetryAfter
		if pause <= 0 {
			pause = quotaPauseDefault
		}
		pause = min(pause, quotaPauseMax)
		slog.Warn("orchestrator: the AI service's quota is used up; roles will wait", "role", job.RoleID, "pause", pause)
		reason := "Waiting: the AI service's usage limit has been reached. The task will be written as soon as it allows — no action is needed."
		if derr := o.DB.DeferRoleTask(context.WithoutCancel(ctx), *job, reason); derr != nil {
			slog.Error("orchestrator: putting a role back in the queue", "role", job.RoleID, "error", derr)
		}
		return true, pause
	}
	if err != nil {
		slog.Error("orchestrator: making a role's task", "role", job.RoleID, "attempt", job.Attempts, "error", err)
		reason := err.Error()
		final := job.Attempts >= db.MaxRoleTaskAttempts
		if final {
			reason = "Task generation failed and will not be retried automatically. " + reason
		}
		if ferr := o.DB.FailRoleTask(context.WithoutCancel(ctx), *job, reason); ferr != nil {
			slog.Error("orchestrator: recording a failed role task", "role", job.RoleID, "error", ferr)
		}
		// Tried once, so this is final: tell the company/admin so a person can
		// fix the brief and save the role again. No candidate is named — the
		// task belongs to the whole role.
		if final {
			o.alertTaskFailure(ctx, job.RoleID, job.CompanyName, job.RoleTitle, "", reason)
		}
		return true, 0
	}
	slog.Info("orchestrator: role task ready", "role", job.RoleID, "took", time.Since(started).Round(time.Second))
	return true, 0
}

// makeRoleTask generates the one task every candidate invited to this role will
// be given, from the role's brief, and stores it against the role.
func (o *Orchestrator) makeRoleTask(ctx context.Context, job db.RoleTaskJob) error {
	// Attribute the generation call to this role in the ai_usage ledger.
	ctx = llm.WithTaskScope(ctx, deref(job.CompanyID), job.RoleID, "")
	notes := strings.TrimSpace(job.Brief)
	if notes == "" {
		notes = "No brief was written for this role; build a task that fits the role's title and tech stack."
	}
	if job.CompanyName != "" {
		notes = "Company: " + job.CompanyName + "\n\n" + notes
	}
	notes += "\n\nThis is the task every candidate invited to this role will be given. Make it a fair, representative test of the role's work at the right level."

	spec := llm.TaskSpec{
		Name: job.RoleTitle, TaskVariant: job.TaskVariant, TechStack: job.TechStack,
		DurationMin: job.DurationMin, Notes: notes,
	}
	task, report, err := o.generateVerifiedTask(ctx, spec, "role "+job.RoleID)
	if err != nil {
		return err
	}
	_, err = o.DB.SaveRoleTask(ctx, job, db.InvitationTask{
		Name: job.RoleTitle, TaskVariant: job.TaskVariant, TechStack: job.TechStack, DurationMin: job.DurationMin,
		TaskBrief: task.TaskBrief, StarterFiles: task.StarterFiles, SolutionFiles: task.SolutionFiles, Verification: report,
	})
	return err
}
