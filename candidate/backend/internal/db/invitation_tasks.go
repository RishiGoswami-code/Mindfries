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
	RoleTitle string
	// CandidateEmail is who the invitation is for — named in the alert sent
	// when the task can't be generated, so a person knows who is blocked.
	CandidateEmail string
	Brief          string
	TaskVariant    string
	TechStack      []string
	DurationMin    int
	Attempts       int
}

// MaxInvitationTaskAttempts is how many times a task is tried before the
// invitation is marked failed for a person to look at. One, deliberately:
// a task is generated exactly once, and if that fails the invitation is
// marked failed and the company/admin is alerted (prepareNextInvitationTask)
// rather than the model being paid to try again and again. Retrying was the
// cost leak — a single doomed task could run up nine full generations and
// still end with nothing. A real failure is now a person's call, not a
// standing order to keep spending.
const MaxInvitationTaskAttempts = 1

// A generation runs for at most orchestrator's invitationTaskCap (15 minutes),
// so one still 'generating' after 20 was cut off — the backend restarted or
// was redeployed mid-run — and nobody is working on it. The claims below give
// such a run one more try even though its attempt was counted (with
// MaxInvitationTaskAttempts at 1, requiring attempts < max, as they once did,
// meant it was never picked up again and the candidate saw "being prepared"
// for ever). FailStalledTasks marks one that stalls a second time as failed.

// ClaimInvitationTask takes the oldest invitation waiting for a task — or
// one whose generation was started so long ago that whoever started it must
// have stopped — and marks it 'generating'. Nil when there is nothing to do.
// Safe with more than one backend instance: a claimed row is skipped by the
// others.
func (d *DB) ClaimInvitationTask(ctx context.Context) (*InvitationTaskJob, error) {
	var job InvitationTaskJob
	err := d.pool.QueryRow(ctx, `
		with next as (
			select a.id from assessments a
			where a.status = 'invited'
			  and ((a.task_status = 'pending' and a.task_attempts < $1)
			       -- A generation cut off by a restart: its attempt was already
			       -- counted, so it gets one more (see FailStalledTasks).
			       or (a.task_status = 'generating' and a.task_started_at < now() - interval '20 minutes'
			           and a.task_attempts < $1 + 1))
			  -- Only invitations whose task isn't being provided at the role
			  -- level: one with no role, or a legacy role that has no role-level
			  -- task ('none'). A role that is generating, has, or failed its own
			  -- task owns its invitations' tasks (role_tasks.go) — the invitation
			  -- worker must not also generate one, or the candidate's task would
			  -- be made twice.
			  and (a.job_role_id is null
			       or exists (select 1 from job_roles r where r.id = a.job_role_id and r.task_status = 'none'))
			order by a.created_at
			limit 1
			for update skip locked
		)
		update assessments a
		set task_status = 'generating', task_started_at = now(), task_attempts = a.task_attempts + 1
		from next
		where a.id = next.id
		returning a.id, a.company_id, a.job_role_id, coalesce(a.role, ''), coalesce(a.candidate_email, ''), a.task_attempts
	`, MaxInvitationTaskAttempts).Scan(&job.ID, &job.CompanyID, &job.RoleID, &job.RoleTitle, &job.CandidateEmail, &job.Attempts)
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

// DeferInvitationTask puts an invitation back in the queue without counting
// this attempt against it — for a failure that is nobody's fault and that
// waiting fixes, like the AI service's quota being used up. The reason is
// recorded so the people who sent the invitation can see why it waits.
func (d *DB) DeferInvitationTask(ctx context.Context, job InvitationTaskJob, reason string) error {
	_, err := d.pool.Exec(ctx, `
		update assessments set task_status = 'pending', task_error = $2, task_attempts = greatest(task_attempts - 1, 0)
		where id = $1 and task_status = 'generating'
	`, job.ID, reason)
	return err
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

// StalledTask is a task generation that was cut off twice and is now marked
// failed, for the worker to alert someone about. CandidateEmail is empty for a
// role's task.
type StalledTask struct {
	ID, RoleTitle, CandidateEmail string
}

// stalledTaskReason is what a twice-interrupted generation records.
const stalledTaskReason = "Task generation failed and will not be retried automatically. It was interrupted twice before it finished (the backend restarted while it ran)."

// FailStalledTasks marks failed every task generation — role or invitation —
// that stalled again after its one extra try, so it says so instead of
// "being prepared" for ever. The company can retry it from its portal.
func (d *DB) FailStalledTasks(ctx context.Context) ([]StalledTask, error) {
	var out []StalledTask
	rows, err := d.pool.Query(ctx, `
		update job_roles
		set task_status = 'failed', task_error = $2
		where task_status = 'generating' and task_started_at < now() - interval '20 minutes'
		  and task_attempts >= $1 + 1
		returning id, coalesce(title, '')
	`, MaxRoleTaskAttempts, stalledTaskReason)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t StalledTask
		if err := rows.Scan(&t.ID, &t.RoleTitle); err != nil {
			rows.Close()
			return out, err
		}
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = d.pool.Query(ctx, `
		update assessments
		set task_status = 'failed', task_error = $2
		where status = 'invited' and task_status = 'generating' and task_started_at < now() - interval '20 minutes'
		  and task_attempts >= $1 + 1
		returning id, coalesce(role, ''), coalesce(candidate_email, '')
	`, MaxInvitationTaskAttempts, stalledTaskReason)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var t StalledTask
		if err := rows.Scan(&t.ID, &t.RoleTitle, &t.CandidateEmail); err != nil {
			return out, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
