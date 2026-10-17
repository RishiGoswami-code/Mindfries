package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/llm"
	"github.com/mindfries/candidate-backend/internal/verify"
)

// Every invitation gets its own task.
//
// A company (from its portal) or Mindfries (from the admin portal) creates
// an invitation for a role; the database marks it 'pending'. This worker
// picks it up, asks the Task Generation agent for a task built from the
// role's brief — told to make it different from the tasks the role's other
// candidates got — runs it to check the tests fail as given and pass once
// solved, and stores it for that invitation alone.
//
// Doing it here, by polling the database, rather than on a request from
// either portal: neither portal has to reach this backend for it, an
// invitation made while this backend was down or redeploying is picked up
// when it is back, and a failed attempt is simply tried again.

const (
	invitationTaskPoll = 10 * time.Second
	// How much of other candidates' briefs the agent is shown, so it can
	// stay clear of them without the prompt growing with every invitation.
	otherBriefsShown  = 3
	otherBriefMaxChar = 1200
	invitationTaskCap = 15 * time.Minute
	// How many times one attempt writes the task before giving up.
	generationsPerAttempt = 3
)

// describeFailure is what the agent is told about its last task: why it
// failed, and the end of what the tests printed both ways.
func describeFailure(r verify.Report) string {
	var b strings.Builder
	b.WriteString("Problem: " + r.Reason + "\n")
	if r.Command != "" {
		b.WriteString("Tests were run with: " + r.Command + "\n")
	}
	tail := func(s string) string {
		if len(s) > 1500 {
			return "…" + s[len(s)-1500:]
		}
		return s
	}
	if r.Starter != nil {
		fmt.Fprintf(&b, "\nWith the starter code (exit code %d):\n%s\n", r.Starter.ExitCode, tail(r.Starter.Output))
	}
	if r.Solution != nil {
		fmt.Fprintf(&b, "\nWith the solution files in place (exit code %d):\n%s\n", r.Solution.ExitCode, tail(r.Solution.Output))
	}
	return b.String()
}

// RunInvitationTaskWorker generates tasks for waiting invitations until ctx
// ends, one at a time.
func (o *Orchestrator) RunInvitationTaskWorker(ctx context.Context) {
	if o.Agents == nil || !o.Agents.Configured() {
		slog.Warn("orchestrator: no AI service is configured; invitations will wait for their tasks")
	}
	for {
		// Work through everything waiting before sleeping again — unless the
		// AI service said its quota is used up, in which case wait as long as
		// it asked (within reason) before trying anything.
		wait := invitationTaskPoll
		for {
			more, pause := o.prepareNextInvitationTask(ctx)
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

// How long the worker waits after the AI service's quota runs out, when
// the service doesn't say: and the longest it waits even when it does, so
// a quota raised meanwhile (billing switched on) is noticed soon.
const (
	quotaPauseDefault = 5 * time.Minute
	quotaPauseMax     = 30 * time.Minute
	busyPause         = 2 * time.Minute
)

// prepareNextInvitationTask handles one waiting invitation. It reports
// whether there was one, and how long to pause first when the AI service's
// quota has run out.
func (o *Orchestrator) prepareNextInvitationTask(ctx context.Context) (bool, time.Duration) {
	if o.Agents == nil || !o.Agents.Configured() {
		return false, 0
	}
	job, err := o.DB.ClaimInvitationTask(ctx)
	if err != nil {
		slog.Error("orchestrator: claiming an invitation to make its task", "error", err)
		return false, 0
	}
	if job == nil {
		return false, 0
	}
	ctx, cancel := context.WithTimeout(ctx, invitationTaskCap)
	defer cancel()
	started := time.Now()
	err = o.makeInvitationTask(ctx, *job)
	var quota *llm.QuotaError
	var busy *llm.BusyError
	if errors.As(err, &busy) {
		// Google was busy or slow even after its retries. Nothing is wrong
		// with this invitation; it goes back in the queue uncounted.
		slog.Warn("orchestrator: the AI service is busy; the invitation will wait", "invitation", job.ID, "pause", busyPause, "error", busy.Message)
		reason := "Waiting: the AI service is busy right now. The task will be written shortly — no action is needed."
		if derr := o.DB.DeferInvitationTask(context.WithoutCancel(ctx), *job, reason); derr != nil {
			slog.Error("orchestrator: putting an invitation back in the queue", "invitation", job.ID, "error", derr)
		}
		return true, busyPause
	}
	if errors.As(err, &quota) {
		pause := quota.RetryAfter
		if pause <= 0 {
			pause = quotaPauseDefault
		}
		pause = min(pause, quotaPauseMax)
		slog.Warn("orchestrator: the AI service's quota is used up; invitations will wait", "invitation", job.ID, "pause", pause)
		reason := "Waiting: the AI service's usage limit has been reached. The task will be written as soon as it allows — no action is needed."
		if derr := o.DB.DeferInvitationTask(context.WithoutCancel(ctx), *job, reason); derr != nil {
			slog.Error("orchestrator: putting an invitation back in the queue", "invitation", job.ID, "error", derr)
		}
		return true, pause
	}
	if err != nil {
		slog.Error("orchestrator: making an invitation's task", "invitation", job.ID, "attempt", job.Attempts, "error", err)
		reason := err.Error()
		if job.Attempts >= db.MaxInvitationTaskAttempts {
			reason = fmt.Sprintf("Gave up after %d attempts. Last problem: %s", job.Attempts, reason)
		}
		if ferr := o.DB.FailInvitationTask(context.WithoutCancel(ctx), *job, reason); ferr != nil {
			slog.Error("orchestrator: recording a failed task", "invitation", job.ID, "error", ferr)
		}
		return true, 0
	}
	slog.Info("orchestrator: invitation task ready", "invitation", job.ID, "took", time.Since(started).Round(time.Second))
	return true, 0
}

func (o *Orchestrator) makeInvitationTask(ctx context.Context, job db.InvitationTaskJob) error {
	var others []string
	if job.RoleID != nil {
		briefs, err := o.DB.OtherBriefsForRole(ctx, *job.RoleID, job.ID, otherBriefsShown)
		if err != nil {
			return fmt.Errorf("reading the role's other tasks: %w", err)
		}
		for _, b := range briefs {
			if len(b) > otherBriefMaxChar {
				b = b[:otherBriefMaxChar] + "…"
			}
			others = append(others, b)
		}
	}

	notes := strings.TrimSpace(job.Brief)
	if notes == "" {
		notes = "No brief was written for this role; build a task that fits the role's title and tech stack."
	}
	if job.CompanyName != "" {
		notes = "Company: " + job.CompanyName + "\n\n" + notes
	}
	notes += "\n\nThis task is for one candidate only. Every candidate for this role gets their own task: test the same skills, at the same level, on a different problem."

	spec := llm.TaskSpec{
		Name: job.RoleTitle, TaskVariant: job.TaskVariant, TechStack: job.TechStack,
		DurationMin: job.DurationMin, Notes: notes,
		DifferentFrom: strings.Join(others, "\n\n---\n\n"),
	}
	// A task that ran and behaved wrongly — tests that pass on the buggy
	// code, or fail on the solution — is not given to anyone. It is written
	// again, with the agent told what went wrong, a few times before this
	// attempt gives up. One that couldn't be run at all is kept, and its
	// verification says so.
	var task llm.GeneratedTask
	var report verify.Report
	for try := 1; ; try++ {
		var err error
		task, err = o.Agents.GenerateTask(ctx, spec)
		if err != nil {
			return fmt.Errorf("the model didn't return a usable task: %w", err)
		}
		report = verify.Check(ctx, o.Verifier, verify.Task{
			StarterFiles:        task.StarterFiles,
			SolutionFiles:       task.SolutionFiles,
			ExpectStarterToPass: strings.Contains(strings.ToLower(job.TaskVariant), "refactor"),
		})
		if report.Status != verify.Failed {
			break
		}
		slog.Info("orchestrator: a generated task didn't check out; writing it again", "invitation", job.ID, "try", try, "reason", report.Reason)
		if try == generationsPerAttempt {
			return fmt.Errorf("the generated task didn't check out: %s", report.Reason)
		}
		spec.PreviousAttempt = describeFailure(report)
	}

	_, err := o.DB.SaveInvitationTask(ctx, job, db.InvitationTask{
		Name: job.RoleTitle, TaskVariant: job.TaskVariant, TechStack: job.TechStack, DurationMin: job.DurationMin,
		TaskBrief: task.TaskBrief, StarterFiles: task.StarterFiles, SolutionFiles: task.SolutionFiles, Verification: report,
	})
	return err
}
