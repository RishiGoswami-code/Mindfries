package support

import (
	"fmt"
	"strings"
)

// Create opens a new ticket with an optional first message from the customer.
func (s *Service) Create(subject, requesterEmail string, priority Priority, firstBody string) (Ticket, error) {
	if strings.TrimSpace(subject) == "" {
		return Ticket{}, fmt.Errorf("subject is required")
	}
	if strings.TrimSpace(requesterEmail) == "" {
		return Ticket{}, fmt.Errorf("requester_email is required")
	}
	if priority == "" {
		priority = PriorityNormal
	}
	now := s.now()
	t := Ticket{
		Subject:        subject,
		RequesterEmail: requesterEmail,
		Status:         StatusOpen,
		Priority:       priority,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if strings.TrimSpace(firstBody) != "" {
		t.Messages = []Message{{AuthorType: AuthorCustomer, Body: firstBody, CreatedAt: now}}
	}
	return s.store.CreateTicket(t)
}

// Reply appends a message. An agent's first reply stamps FirstResponse (the SLA
// clock). A customer replying to a resolved ticket reopens it.
func (s *Service) Reply(ticketID string, author AuthorType, authorID, body string) (Ticket, error) {
	if strings.TrimSpace(body) == "" {
		return Ticket{}, fmt.Errorf("message body is required")
	}
	t, err := s.store.GetTicket(ticketID)
	if err != nil {
		return Ticket{}, err
	}
	now := s.now()
	t.Messages = append(t.Messages, Message{
		TicketID:   t.ID,
		AuthorType: author,
		AuthorID:   authorID,
		Body:       body,
		CreatedAt:  now,
	})
	t.UpdatedAt = now

	if author == AuthorAgent && t.FirstResponse == nil {
		fr := now
		t.FirstResponse = &fr
	}
	if author == AuthorCustomer && (t.Status == StatusResolved || t.Status == StatusClosed) {
		t.Status = StatusOpen // reopen
		t.ResolvedAt = nil
	}
	return s.store.UpdateTicket(t)
}

// Assign puts a ticket on an agent (and that agent's team).
func (s *Service) Assign(ticketID, agentID string) (Ticket, error) {
	t, err := s.store.GetTicket(ticketID)
	if err != nil {
		return Ticket{}, err
	}
	a, err := s.store.GetAgent(agentID)
	if err != nil {
		return Ticket{}, err
	}
	t.AssigneeID = a.ID
	t.TeamID = a.TeamID
	t.UpdatedAt = s.now()
	return s.store.UpdateTicket(t)
}

// SetStatus moves a ticket's status, stamping ResolvedAt on resolution and
// clearing it on reopen.
func (s *Service) SetStatus(ticketID string, status Status) (Ticket, error) {
	switch status {
	case StatusOpen, StatusPending, StatusResolved, StatusClosed:
	default:
		return Ticket{}, fmt.Errorf("unknown status %q", status)
	}
	t, err := s.store.GetTicket(ticketID)
	if err != nil {
		return Ticket{}, err
	}
	now := s.now()
	if status == StatusResolved && t.ResolvedAt == nil {
		t.ResolvedAt = &now
	}
	if status == StatusOpen {
		t.ResolvedAt = nil
	}
	t.Status = status
	t.UpdatedAt = now
	return s.store.UpdateTicket(t)
}

// Rate records a 1–5 CSAT score; only a resolved or closed ticket can be rated.
func (s *Service) Rate(ticketID string, score int) (Ticket, error) {
	if score < 1 || score > 5 {
		return Ticket{}, fmt.Errorf("satisfaction must be 1–5, got %d", score)
	}
	t, err := s.store.GetTicket(ticketID)
	if err != nil {
		return Ticket{}, err
	}
	if t.Status != StatusResolved && t.Status != StatusClosed {
		return Ticket{}, fmt.Errorf("only a resolved or closed ticket can be rated (status %s)", t.Status)
	}
	t.Satisfaction = score
	t.UpdatedAt = s.now()
	return s.store.UpdateTicket(t)
}
