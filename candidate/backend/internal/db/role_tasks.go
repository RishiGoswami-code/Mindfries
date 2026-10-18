package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// A job role's own task (migration 0028). A company describes a role once, so
// its task is generated once — when the role is created or its brief is
// edited — and shared by everyone invited to it, rather than regenerated for
// each candidate. This is the role-level counterpart to invitation_tasks.go.

// RoleTaskJob is a role waiting for its task, with what to generate it from.
type RoleTaskJob struct {
	RoleID      string
	CompanyID   *string
	CompanyName string
	RoleTitle   string
	Brief       string
	TaskVariant string
	TechStack   []string
	DurationMin int
	Attempts    int
}

// MaxRoleTaskAttempts is how many times a role's task is tried before it is
// marked failed for a person to look at. One, like an invitation's: generated
// once, and a failure is the company/admin's call to make, not a standing
// order to keep paying the model to retry (see invitation_tasks.go).
const MaxRoleTaskAttempts = 1

// ClaimRoleTask takes the oldest role waiting for a task — or one whose
// generation stalled — and marks it 'generating'. Nil when there is nothing to
// do. Safe with more than one backend instance (for update skip locked).
func (d *DB) ClaimRoleTask(ctx context.Context) (*RoleTaskJob, error) {
	var job RoleTaskJob
	var duration *int
	err := d.pool.QueryRow(ctx, `
		with next as (
			select id from job_roles
			where task_attempts < $1
			  and (task_status = 'pending'
			       or (task_status = 'generating' and task_started_at < now() - interval '15 minutes'))
			order by created_at
			limit 1
			for update skip locked
		)
		update job_roles r
		set task_status = 'generating', task_started_at = now(), task_attempts = r.task_attempts + 1
		from next
		where r.id = next.id
		returning r.id, r.company_id, coalesce(r.title, ''), coalesce(r.assessment_brief, ''),
		          coalesce(r.task_variant, 'bug_fix'), coalesce(r.tech_stack, '{}'), r.duration_min, r.task_attempts
	`, MaxRoleTaskAttempts).Scan(&job.RoleID, &job.CompanyID, &job.RoleTitle, &job.Brief,
		&job.TaskVariant, &job.TechStack, &duration, &job.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.DurationMin = 45
	if duration != nil && *duration > 0 {
		job.DurationMin = *duration
	}
	if job.RoleTitle == "" {
		job.RoleTitle = "Software engineer"
	}
	if job.CompanyID != nil {
		_ = d.pool.QueryRow(ctx, `select coalesce(name, '') from companies where id = $1`, *job.CompanyID).Scan(&job.CompanyName)
	}
	return &job, nil
}

// SaveRoleTask stores the generated task as a template private to the role's
// company, links it to the role (which becomes 'ready'), and backfills any
// invitations already waiting on this role so they pick up the task too —
// without a candidate ever having watched a "preparing" screen. All or none.
func (d *DB) SaveRoleTask(ctx context.Context, job RoleTaskJob, task InvitationTask) (string, error) {
	starter, err := json.Marshal(task.StarterFiles)
	if err != nil {
		return "", err
	}
	solution, err := json.Marshal(task.SolutionFiles)
	if err != nil {
		return "", err
	}
	verification, err := json.Marshal(task.Verification)
	if err != nil {
		return "", err
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var templateID string
	err = tx.QueryRow(ctx, `
		insert into game_templates (name, task_variant, tech_stack, duration_min, task_brief, starter_files, solution_files, verification, company_id, status)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'invitation')
		returning id
	`, task.Name, task.TaskVariant, task.TechStack, task.DurationMin, task.TaskBrief, starter, solution, verification, job.CompanyID).Scan(&templateID)
	if err != nil {
		return "", err
	}
	tag, err := tx.Exec(ctx, `
		update job_roles set template_id = $2, task_status = 'ready', task_error = null
		where id = $1 and task_status = 'generating'
	`, job.RoleID, templateID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		// The role changed (another regeneration, or it was deleted) meanwhile.
		return "", errors.New("db: the role changed while its task was being made")
	}
	// Invitations sent before the role's task was ready: give them the task now,
	// so they stop showing "preparing". Only those that don't already have one.
	if _, err := tx.Exec(ctx, `
		update assessments set template_id = $2, task_status = 'ready', task_error = null
		where job_role_id = $1 and template_id is null and task_status in ('pending', 'generating')
	`, job.RoleID, templateID); err != nil {
		return "", err
	}
	return templateID, tx.Commit(ctx)
}

// FailRoleTask records why a role's task couldn't be generated. The role goes
// to 'failed' once it has had MaxRoleTaskAttempts; the company/admin is alerted
// by the worker so a person can fix the brief and save the role again.
func (d *DB) FailRoleTask(ctx context.Context, job RoleTaskJob, reason string) error {
	status := "pending"
	if job.Attempts >= MaxRoleTaskAttempts {
		status = "failed"
	}
	_, err := d.pool.Exec(ctx, `
		update job_roles set task_status = $2, task_error = $3
		where id = $1 and task_status = 'generating'
	`, job.RoleID, status, reason)
	return err
}

// DeferRoleTask puts a role's task back in the queue without counting the
// attempt — for a failure that waiting fixes, like the AI service's quota
// being used up.
func (d *DB) DeferRoleTask(ctx context.Context, job RoleTaskJob, reason string) error {
	_, err := d.pool.Exec(ctx, `
		update job_roles set task_status = 'pending', task_error = $2, task_attempts = greatest(task_attempts - 1, 0)
		where id = $1 and task_status = 'generating'
	`, job.RoleID, reason)
	return err
}
