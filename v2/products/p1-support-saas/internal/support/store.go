package support

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ErrNotFound is returned when a looked-up entity does not exist.
var ErrNotFound = errors.New("not found")

// TicketFilter narrows a ListTickets query. Zero-value fields are ignored.
type TicketFilter struct {
	Status     Status
	Priority   Priority
	AssigneeID string
	TeamID     string
	// Unassigned, when true, matches only tickets with no assignee. (AssigneeID
	// still takes precedence if both are set.)
	Unassigned bool
}

// Store holds the product's data. In production this would be Postgres; the
// in-memory implementation keeps the product runnable and the test suite
// dependency-free, behind an interface a candidate task could swap.
type Store interface {
	CreateTicket(t Ticket) (Ticket, error)
	GetTicket(id string) (Ticket, error)
	UpdateTicket(t Ticket) (Ticket, error)
	ListTickets(f TicketFilter) ([]Ticket, error)

	Agents() []Agent
	GetAgent(id string) (Agent, error)
	Teams() []Team
	SLAPolicies() []SLAPolicy
	KBArticles() []KBArticle

	Seed(agents []Agent, teams []Team, policies []SLAPolicy, kb []KBArticle)
}

type memStore struct {
	mu       sync.RWMutex
	tickets  map[string]Ticket
	order    []string // ticket ids in creation order, for stable listing
	agents   map[string]Agent
	teams    map[string]Team
	policies []SLAPolicy
	kb       []KBArticle
	seq      map[string]int // id counters per prefix
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() Store {
	return &memStore{
		tickets: map[string]Ticket{},
		agents:  map[string]Agent{},
		teams:   map[string]Team{},
		seq:     map[string]int{},
	}
}

// nextID returns a deterministic, monotonic id like "TCK-1". Deterministic ids
// keep tests readable and avoid a uuid dependency.
func (s *memStore) nextID(prefix string) string {
	s.seq[prefix]++
	return fmt.Sprintf("%s-%d", prefix, s.seq[prefix])
}

func (s *memStore) CreateTicket(t Ticket) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = s.nextID("TCK")
	}
	for i := range t.Messages {
		if t.Messages[i].ID == "" {
			t.Messages[i].ID = s.nextID("MSG")
		}
		t.Messages[i].TicketID = t.ID
	}
	s.tickets[t.ID] = t
	s.order = append(s.order, t.ID)
	return t, nil
}

func (s *memStore) GetTicket(id string) (Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tickets[id]
	if !ok {
		return Ticket{}, fmt.Errorf("ticket %s: %w", id, ErrNotFound)
	}
	return t, nil
}

func (s *memStore) UpdateTicket(t Ticket) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tickets[t.ID]; !ok {
		return Ticket{}, fmt.Errorf("ticket %s: %w", t.ID, ErrNotFound)
	}
	for i := range t.Messages {
		if t.Messages[i].ID == "" {
			t.Messages[i].ID = s.nextID("MSG")
		}
		t.Messages[i].TicketID = t.ID
	}
	s.tickets[t.ID] = t
	return t, nil
}

func (s *memStore) ListTickets(f TicketFilter) ([]Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Ticket
	for _, id := range s.order {
		t := s.tickets[id]
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.Priority != "" && t.Priority != f.Priority {
			continue
		}
		if f.AssigneeID != "" && t.AssigneeID != f.AssigneeID {
			continue
		}
		if f.AssigneeID == "" && f.Unassigned && t.AssigneeID != "" {
			continue
		}
		if f.TeamID != "" && t.TeamID != f.TeamID {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *memStore) Agents() []Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Agent, 0, len(s.agents))
	for _, a := range s.agents {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *memStore) GetAgent(id string) (Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.agents[id]
	if !ok {
		return Agent{}, fmt.Errorf("agent %s: %w", id, ErrNotFound)
	}
	return a, nil
}

func (s *memStore) Teams() []Team {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Team, 0, len(s.teams))
	for _, t := range s.teams {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *memStore) SLAPolicies() []SLAPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]SLAPolicy(nil), s.policies...)
}

func (s *memStore) KBArticles() []KBArticle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]KBArticle(nil), s.kb...)
}

func (s *memStore) Seed(agents []Agent, teams []Team, policies []SLAPolicy, kb []KBArticle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range agents {
		s.agents[a.ID] = a
	}
	for _, t := range teams {
		s.teams[t.ID] = t
	}
	s.policies = append(s.policies, policies...)
	s.kb = append(s.kb, kb...)
}
