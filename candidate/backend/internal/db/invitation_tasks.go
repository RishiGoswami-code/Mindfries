package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// An invitation's own task (migration 0022). Each invitation is created
// without one, marked 'pending'; candidate/backend claims it, generates a
// task for that candidate from the role's brief, and links it — or records
// why it couldn't.

// InvitationTaskJob is an invitation waiting for its task, with what the
// task is to be generated from.
type InvitationTaskJob struct {
	ID          string
	CompanyID   *string
	CompanyName string
	RoleID      *string
	// RoleTitle is the role's title, or the invitation's own role text for an
	// invitation that isn't attached to a role.
	RoleTitle   string
	Brief       string
	TaskVariant string
	TechStack   []string
	DurationMin int
	Attempts    int
}

// MaxInvitationTaskAttempts is how many times a task is tried before the
// invitation is marked failed for a person to look at.
const MaxInvitationTaskAttempts = 3

// ClaimInvitationTask takes the oldest invitation waiting for a task — or
// one whose generation was started so long ago that whoever started it must
// have stopped — and marks it 'generating'. Nil when there is nothing to do.
// Safe with more than one backend instance: a claimed row is skipped by the
// others.
func (d *DB) ClaimInvitationTask(ctx context.Context) (*InvitationTaskJob, error) {
	var job InvitationTaskJob
	err := d.pool.QueryRow(ctx, `
		with next as (
			select id from assessments
			where status = 'invited'
			  and task_attempts < $1
			  and (task_status = 'pending'
			       or (task_status = 'generating' and task_started_at < now() - interval '15 minutes'))
			order by created_at
			limit 1
			for update skip locked
		)
		update assessments a
		set task_status = 'generating', task_started_at = now(), task_attempts = a.task_attempts + 1
		from next
		where a.id = next.id
		returning a.id, a.company_id, a.job_role_id, coalesce(a.role, ''), a.task_attempts
	`, MaxInvitationTaskAttempts).Scan(&job.ID, &job.CompanyID, &job.RoleID, &job.RoleTitle, &job.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	job.TaskVariant, job.DurationMin = "bug_fix", 45
	if job.CompanyID != nil {
		_ = d.pool.QueryRow(ctx, `select coalesce(name, '') from companies where id = $1`, *job.CompanyID).Scan(&job.CompanyName)
	}
	if job.RoleID != nil {
		var title string
		var duration *int
		err := d.pool.QueryRow(ctx, `
			select title, coalesce(assessment_brief, ''), coalesce(task_variant, 'bug_fix'), coalesce(tech_stack, '{}'), duration_min
			from job_roles where id = $1
		`, *job.RoleID).Scan(&title, &job.Brief, &job.TaskVariant, &job.TechStack, &duration)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return &job, err
		}
		if title != "" {
			job.RoleTitle = title
		}
		if duration != nil && *duration > 0 {
			job.DurationMin = *duration
		}
	}
	if job.RoleTitle == "" {
		job.RoleTitle = "Software engineer"
	}
	return &job, nil
}

// OtherBriefsForRole is the task briefs already given to other candidates
// for a role, newest first — what a new candidate's task must differ from.
func (d *DB) OtherBriefsForRole(ctx context.Context, roleID, exceptInvitationID string, limit int) ([]string, error) {
	rows, err := d.pool.Query(ctx, `
		select t.task_brief
		from assessments a join game_templates t on t.id = a.template_id
		where a.job_role_id = $1 and a.id <> $2 and coalesce(t.task_brief, '') <> ''
		order by a.created_at desc
		limit $3
	`, roleID, exceptInvitationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var brief string
		if err := rows.Scan(&brief); err != nil {
			return nil, err
		}
		out = append(out, brief)
	}
	return out, rows.Err()
}

// InvitationTask is a generated task, ready to be stored for one invitation.
type InvitationTask struct {
	Name          string
	TaskVariant   string
	TechStack     []string
	DurationMin   int
	TaskBrief     string
	StarterFiles  map[string]string
	SolutionFiles map[string]string
	Verification  any
}

// SaveInvitationTask stores the task as a template private to the
// invitation's company (status 'invitation', so it is never listed as a
// library task) and links it to the invitation, which becomes 'ready'.
// Both or neither.
func (d *DB) SaveInvitationTask(ctx context.Context, job InvitationTaskJob, task InvitationTask) (string, error) {
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
		update assessments set template_id = $2, task_status = 'ready', task_error = null
		where id = $1 and task_status = 'generating'
	`, job.ID, templateID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		// Someone reset or withdrew the invitation meanwhile; leave it be.
		return "", errors.New("db: the invitation changed while its task was being made")
	}
	return templateID, tx.Commit(ctx)
}

// FailInvitationTask records why this attempt didn't produce a task. The
// invitation goes back to 'pending' for another try, or to 'failed' once it
// has had MaxInvitationTaskAttempts.
func (d *DB) FailInvitationTask(ctx context.Context, job InvitationTaskJob, reason string) error {
	status := "pending"
	if job.Attempts >= MaxInvitationTaskAttempts {
		status = "failed"
	}
	_, err := d.pool.Exec(ctx, `
		update assessments set task_status = $2, task_error = $3
		where id = $1 and task_status = 'generating'
	`, job.ID, status, reason)
	return err
}
