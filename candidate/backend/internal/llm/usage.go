package llm

import "context"

// Usage is what one model call cost, as OpenRouter reported it. Every call
// produces one, so spend can be attributed to the session — and through it,
// the candidate and company — that caused it, rather than only being visible
// as a running total on OpenRouter's dashboard.
type Usage struct {
	// Agent is which of this package's agents made the call, e.g.
	// "assistant" or "code_evaluation".
	Agent string `json:"agent"`
	// SessionID is the assessment session the call was made for. Empty for
	// calls that belong to no session (task generation in the admin library).
	SessionID string `json:"-"`
	Model     string `json:"model"`
	// PromptTokens and CompletionTokens are OpenRouter's own counts.
	// CompletionTokens includes any reasoning tokens — they're billed as output.
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	// CachedTokens is how many of PromptTokens were served from a context
	// cache at a discount (Gemini's cachedContentTokenCount / OpenRouter's
	// cached prompt tokens). It is already included in PromptTokens — it's
	// surfaced on its own so a session's cache-hit rate is visible, which is
	// what tells us whether prefix caching is actually saving anything.
	CachedTokens int `json:"cachedTokens"`
	// CostUSD is the charge for this call in US dollars, as billed.
	CostUSD float64 `json:"costUsd"`
	// What the call was for when it belongs to no session — task generation
	// for a role or an invitation. These let the sessionless calls (which the
	// per-session ai_cost trail can't hold) be attributed in the ai_usage
	// ledger, so it's visible which role or task is spending the API.
	CompanyID    string `json:"-"`
	RoleID       string `json:"-"`
	InvitationID string `json:"-"`
}

type ctxKey int

const (
	ctxSession ctxKey = iota
	ctxAgent
	ctxScope
)

// taskScope is what a sessionless call (task generation) is for.
type taskScope struct{ company, role, invitation string }

// WithSession tags every model call made with ctx as belonging to a session.
func WithSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, ctxSession, sessionID)
}

// WithTaskScope tags a sessionless call (task generation) with what it's for,
// so its usage can be attributed to the role or invitation in the ai_usage
// ledger. Any of the ids may be empty.
func WithTaskScope(ctx context.Context, companyID, roleID, invitationID string) context.Context {
	return context.WithValue(ctx, ctxScope, taskScope{company: companyID, role: roleID, invitation: invitationID})
}

// withAgent names the agent making the calls under ctx. Set by each agent
// method itself, so a caller can't mislabel one.
func withAgent(ctx context.Context, agent string) context.Context {
	return context.WithValue(ctx, ctxAgent, agent)
}

func tagsFrom(ctx context.Context) (agent, sessionID string) {
	agent, _ = ctx.Value(ctxAgent).(string)
	sessionID, _ = ctx.Value(ctxSession).(string)
	return agent, sessionID
}

func scopeFrom(ctx context.Context) (company, role, invitation string) {
	if s, ok := ctx.Value(ctxScope).(taskScope); ok {
		return s.company, s.role, s.invitation
	}
	return "", "", ""
}
