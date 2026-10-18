package db

import "context"

// AIUsage is one model call for the ai_usage ledger (migration 0029). The id
// fields are empty when they don't apply — a session call has SessionID, a
// task-generation call has RoleID and/or InvitationID.
type AIUsage struct {
	Agent            string
	Model            string
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	CostUSD          float64
	SessionID        string
	CompanyID        string
	RoleID           string
	InvitationID     string
}

// InsertAIUsage appends one call to the ledger. Empty id fields are stored as
// NULL (a "" would be an invalid uuid).
func (d *DB) InsertAIUsage(ctx context.Context, u AIUsage) error {
	_, err := d.pool.Exec(ctx, `
		insert into ai_usage (agent, model, prompt_tokens, completion_tokens, cached_tokens, cost_usd, session_id, company_id, role_id, invitation_id)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, u.Agent, u.Model, u.PromptTokens, u.CompletionTokens, u.CachedTokens, u.CostUSD,
		nilIfEmpty(u.SessionID), nilIfEmpty(u.CompanyID), nilIfEmpty(u.RoleID), nilIfEmpty(u.InvitationID))
	return err
}

// nilIfEmpty passes an empty string to the database as NULL — needed for uuid
// columns, which reject "".
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
