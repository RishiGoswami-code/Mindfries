package support

import "time"

// policyFor returns the SLA policy matching a ticket's priority.
func (s *Service) policyFor(p Priority) (SLAPolicy, bool) {
	for _, pol := range s.store.SLAPolicies() {
		if pol.Priority == p {
			return pol, true
		}
	}
	return SLAPolicy{}, false
}

// FirstResponseDue is the deadline for the first agent reply, measured from the
// ticket's creation. The bool is false when no policy covers the priority.
func (s *Service) FirstResponseDue(t Ticket) (time.Time, bool) {
	pol, ok := s.policyFor(t.Priority)
	if !ok {
		return time.Time{}, false
	}
	return t.CreatedAt.Add(pol.FirstResponseTime), true
}

// ResolutionDue is the deadline for resolving the ticket.
func (s *Service) ResolutionDue(t Ticket) (time.Time, bool) {
	pol, ok := s.policyFor(t.Priority)
	if !ok {
		return time.Time{}, false
	}
	return t.CreatedAt.Add(pol.ResolutionTime), true
}

// FirstResponseBreached reports whether the first-response SLA is missed: the
// agent replied after the deadline, or no reply yet and the deadline has passed.
func (s *Service) FirstResponseBreached(t Ticket) bool {
	due, ok := s.FirstResponseDue(t)
	if !ok {
		return false
	}
	if t.FirstResponse != nil {
		return t.FirstResponse.After(due)
	}
	return s.now().After(due)
}

// ResolutionBreached reports whether the resolution SLA is missed: resolved
// after the deadline, or not resolved and the deadline has passed.
func (s *Service) ResolutionBreached(t Ticket) bool {
	due, ok := s.ResolutionDue(t)
	if !ok {
		return false
	}
	if t.ResolvedAt != nil {
		return t.ResolvedAt.After(due)
	}
	if t.Status == StatusClosed {
		return false // closed without a resolve stamp: treat as not breached
	}
	return s.now().After(due)
}

// BreachedTickets returns the open tickets currently in breach of either SLA —
// the queue an agent or the FDE triage view works from.
func (s *Service) BreachedTickets() ([]Ticket, error) {
	all, err := s.store.ListTickets(TicketFilter{})
	if err != nil {
		return nil, err
	}
	var out []Ticket
	for _, t := range all {
		if t.Status == StatusClosed {
			continue
		}
		if s.FirstResponseBreached(t) || s.ResolutionBreached(t) {
			out = append(out, t)
		}
	}
	return out, nil
}
