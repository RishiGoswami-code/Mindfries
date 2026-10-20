package support

import (
	"fmt"
	"sort"
)

// agentLoad counts a given agent's unresolved (open or pending) tickets.
func (s *Service) agentLoad() (map[string]int, error) {
	load := map[string]int{}
	all, err := s.store.ListTickets(TicketFilter{})
	if err != nil {
		return nil, err
	}
	for _, t := range all {
		if t.AssigneeID == "" {
			continue
		}
		if t.Status == StatusOpen || t.Status == StatusPending {
			load[t.AssigneeID]++
		}
	}
	return load, nil
}

// AutoAssign puts the ticket on the least-loaded active agent, preferring the
// ticket's team when it has one. Ties break by agent id so the choice is
// deterministic. This is the backbone of fair queue distribution.
func (s *Service) AutoAssign(ticketID string) (Ticket, error) {
	t, err := s.store.GetTicket(ticketID)
	if err != nil {
		return Ticket{}, err
	}

	load, err := s.agentLoad()
	if err != nil {
		return Ticket{}, err
	}

	candidates := make([]Agent, 0)
	for _, a := range s.store.Agents() {
		if !a.Active {
			continue
		}
		if t.TeamID != "" && a.TeamID != t.TeamID {
			continue
		}
		candidates = append(candidates, a)
	}
	// If the team has no active agents, fall back to any active agent.
	if len(candidates) == 0 {
		for _, a := range s.store.Agents() {
			if a.Active {
				candidates = append(candidates, a)
			}
		}
	}
	if len(candidates) == 0 {
		return Ticket{}, fmt.Errorf("no active agent available to assign")
	}

	sort.Slice(candidates, func(i, j int) bool {
		li, lj := load[candidates[i].ID], load[candidates[j].ID]
		if li != lj {
			return li < lj
		}
		return candidates[i].ID < candidates[j].ID
	})
	return s.Assign(t.ID, candidates[0].ID)
}
