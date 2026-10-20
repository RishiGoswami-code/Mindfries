// Package support is the domain of the Helpdesk product: tickets and the
// conversation on them, the agents and teams that handle them, the SLA
// policies they're held to, and the knowledge base the suggested-reply feature
// draws on. The HTTP API (internal/httpapi) and the server (cmd/server) are
// thin layers over this package.
package support

import "time"

// Status is where a ticket is in its lifecycle.
type Status string

const (
	StatusOpen     Status = "open"     // awaiting an agent
	StatusPending  Status = "pending"  // waiting on the customer
	StatusResolved Status = "resolved" // fixed, not yet confirmed closed
	StatusClosed   Status = "closed"   // done
)

// Priority drives SLA targets and routing.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// AuthorType says who wrote a message.
type AuthorType string

const (
	AuthorCustomer AuthorType = "customer"
	AuthorAgent    AuthorType = "agent"
	AuthorSystem   AuthorType = "system"
)

// Ticket is one customer support request.
type Ticket struct {
	ID             string     `json:"id"`
	Subject        string     `json:"subject"`
	RequesterEmail string     `json:"requester_email"`
	Status         Status     `json:"status"`
	Priority       Priority   `json:"priority"`
	AssigneeID     string     `json:"assignee_id,omitempty"`
	TeamID         string     `json:"team_id,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	Messages       []Message  `json:"messages"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	FirstResponse  *time.Time `json:"first_response_at,omitempty"` // first agent reply
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	// Satisfaction is a 1–5 CSAT rating left by the customer after resolution,
	// 0 if none. Fed into metrics.
	Satisfaction int `json:"satisfaction,omitempty"`
}

// Message is one entry in a ticket's conversation.
type Message struct {
	ID         string     `json:"id"`
	TicketID   string     `json:"ticket_id"`
	AuthorType AuthorType `json:"author_type"`
	AuthorID   string     `json:"author_id,omitempty"` // agent id when AuthorAgent
	Body       string     `json:"body"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Agent handles tickets; belongs to a team.
type Agent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	TeamID string `json:"team_id"`
	Active bool   `json:"active"`
}

// Team groups agents.
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SLAPolicy sets response and resolution targets for a priority.
type SLAPolicy struct {
	ID                string        `json:"id"`
	Priority          Priority      `json:"priority"`
	FirstResponseTime time.Duration `json:"first_response_time"`
	ResolutionTime    time.Duration `json:"resolution_time"`
}

// KBArticle is a knowledge-base entry the suggested-reply feature retrieves.
type KBArticle struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}
