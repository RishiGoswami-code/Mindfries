// Package seed loads realistic sample data so the product looks like a live
// support desk the moment a candidate opens it: real teams and agents, SLA
// policies, a small knowledge base, and a spread of tickets across statuses,
// priorities, and ages (some already in SLA breach).
package seed

import (
	"time"

	"github.com/mindfries/products/p1-support-saas/internal/support"
)

// Catalog is the reference (agents/teams/SLA/KB) seeded into the store.
func Catalog() (agents []support.Agent, teams []support.Team, policies []support.SLAPolicy, kb []support.KBArticle) {
	teams = []support.Team{
		{ID: "team-support", Name: "Customer Support"},
		{ID: "team-billing", Name: "Billing"},
	}
	agents = []support.Agent{
		{ID: "agent-ada", Name: "Ada Lovelace", TeamID: "team-support", Active: true},
		{ID: "agent-ben", Name: "Ben Carter", TeamID: "team-support", Active: true},
		{ID: "agent-cleo", Name: "Cleo Nguyen", TeamID: "team-billing", Active: true},
		{ID: "agent-dmitri", Name: "Dmitri Volkov", TeamID: "team-billing", Active: false},
	}
	policies = []support.SLAPolicy{
		{ID: "sla-urgent", Priority: support.PriorityUrgent, FirstResponseTime: 1 * time.Hour, ResolutionTime: 4 * time.Hour},
		{ID: "sla-high", Priority: support.PriorityHigh, FirstResponseTime: 2 * time.Hour, ResolutionTime: 12 * time.Hour},
		{ID: "sla-normal", Priority: support.PriorityNormal, FirstResponseTime: 8 * time.Hour, ResolutionTime: 48 * time.Hour},
		{ID: "sla-low", Priority: support.PriorityLow, FirstResponseTime: 24 * time.Hour, ResolutionTime: 120 * time.Hour},
	}
	kb = []support.KBArticle{
		{ID: "kb-password", Title: "Resetting your password", Body: "Use the forgot-password link on the login page to receive a reset email and restore access.", Tags: []string{"password", "login", "access"}},
		{ID: "kb-2fa", Title: "Setting up two-factor authentication", Body: "Enable 2FA under security settings using an authenticator app for stronger account security.", Tags: []string{"security", "login", "authentication"}},
		{ID: "kb-invoice", Title: "Finding your invoices", Body: "Download past invoices and receipts from the billing page under your account settings.", Tags: []string{"billing", "invoice", "payment"}},
		{ID: "kb-refund", Title: "Requesting a refund", Body: "Refunds for the current billing period can be requested from the billing page within 30 days.", Tags: []string{"billing", "refund", "payment"}},
		{ID: "kb-export", Title: "Exporting your data", Body: "Export your workspace data as CSV from the admin data export panel.", Tags: []string{"data", "export", "admin"}},
	}
	return
}

// Load seeds the catalog and a spread of sample tickets into svc's store,
// using base as "now minus the ticket's age" so some tickets are already in
// SLA breach. It returns the created tickets.
func Load(svc *support.Service, base time.Time) []support.Ticket {
	var created []support.Ticket

	type spec struct {
		subject, email, body string
		priority             support.Priority
		assignee             string
		resolve              bool
		csat                 int
	}
	specs := []spec{
		{"Can't log in to my account", "jo@acme.test", "I keep getting an invalid password error when I reset my password.", support.PriorityUrgent, "agent-ada", false, 0},
		{"Where do I download my invoice?", "mei@globex.test", "I need last month's invoice for expenses.", support.PriorityNormal, "agent-cleo", true, 5},
		{"Refund for duplicate charge", "sam@initech.test", "I was billed twice this month and need a refund.", support.PriorityHigh, "agent-cleo", false, 0},
		{"How do I enable 2FA?", "pat@umbrella.test", "I want to turn on two-factor authentication for security.", support.PriorityLow, "", false, 0},
		{"Data export is failing", "lee@hooli.test", "The CSV export from the admin panel returns an error.", support.PriorityHigh, "agent-ben", true, 4},
	}

	for i, sp := range specs {
		// Space tickets out so ages vary; the urgent unassigned-reply one breaches.
		age := time.Duration(i+1) * 3 * time.Hour
		svc2 := support.NewService(svc.Store(), func() time.Time { return base.Add(-age) })
		t, _ := svc2.Create(sp.subject, sp.email, sp.priority, sp.body)
		if sp.assignee != "" {
			t, _ = svc2.Assign(t.ID, sp.assignee)
		}
		if sp.resolve {
			svc3 := support.NewService(svc.Store(), func() time.Time { return base.Add(-age + time.Hour) })
			svc3.Reply(t.ID, support.AuthorAgent, sp.assignee, "Thanks for reaching out — taking a look now.")
			t, _ = svc3.SetStatus(t.ID, support.StatusResolved)
			if sp.csat > 0 {
				t, _ = svc3.Rate(t.ID, sp.csat)
			}
		}
		created = append(created, t)
	}
	return created
}
