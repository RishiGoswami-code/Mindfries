package orchestrator

import (
	"context"
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
	invitationTaskCap = 10 * time.Minute
)

// RunInvitationTaskWorker generates tasks for waiting invitations until ctx
// ends, one at a time.
func (o *Orchestrator) RunInvitationTaskWorker(ctx context.Context) {
	if o.Agents == nil || !o.Agents.Configured() {
		slog.Warn("orchestrator: no AI service is configured; invitations will wait for their tasks")
	}
	ticker := time.NewTicker(invitationTaskPoll)
	defer ticker.Stop()
	for {
		// Work through everything waiting before sleeping again.
		for o.prepareNextInvitationTask(ctx) {
			if ctx.Err() != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// prepareNextInvitationTask handles one waiting invitation. It reports
// whether there was one.
func (o *Orchestrator) prepareNextInvitationTask(ctx context.Context) bool {
	if o.Agents == nil || !o.Agents.Configured() {
		return false
	}
	job, err := o.DB.ClaimInvitationTask(ctx)
	if err != nil {
		slog.Error("orchestrator: claiming an invitation to make its task", "error", err)
		return false
	}
	if job == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, invitationTaskCap)
	defer cancel()
	started := time.Now()
	if err := o.makeInvitationTask(ctx, *job); err != nil {
		slog.Error("orchestrator: making an invitation's task", "invitation", job.ID, "attempt", job.Attempts, "error", err)
		reason := err.Error()
		if job.Attempts >= db.MaxInvitationTaskAttempts {
			reason = fmt.Sprintf("Gave up after %d attempts. Last problem: %s", job.Attempts, reason)
		}
		if ferr := o.DB.FailInvitationTask(context.WithoutCancel(ctx), *job, reason); ferr != nil {
			slog.Error("orchestrator: recording a failed task", "invitation", job.ID, "error", ferr)
		}
		return true
	}
	slog.Info("orchestrator: invitation task ready", "invitation", job.ID, "took", time.Since(started).Round(time.Second))
	return true
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
	task, err := o.Agents.GenerateTask(ctx, spec)
	if err != nil {
		return fmt.Errorf("the model didn't return a usable task: %w", err)
	}

	report := verify.Check(ctx, o.Verifier, verify.Task{
		StarterFiles:        task.StarterFiles,
		SolutionFiles:       task.SolutionFiles,
		ExpectStarterToPass: strings.Contains(strings.ToLower(job.TaskVariant), "refactor"),
	})
	// A task that ran and behaved wrongly — tests that pass on the buggy
	// code, or fail on the solution — is not given to anyone. One that
	// couldn't be run at all is kept, and its verification says so.
	if report.Status == verify.Failed {
		return fmt.Errorf("the generated task didn't check out: %s", report.Reason)
	}

	_, err = o.DB.SaveInvitationTask(ctx, job, db.InvitationTask{
		Name: job.RoleTitle, TaskVariant: job.TaskVariant, TechStack: job.TechStack, DurationMin: job.DurationMin,
		TaskBrief: task.TaskBrief, StarterFiles: task.StarterFiles, SolutionFiles: task.SolutionFiles, Verification: report,
	})
	return err
}
