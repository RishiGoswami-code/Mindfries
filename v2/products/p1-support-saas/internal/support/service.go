package support

import "time"

// Service is the application layer: all ticket operations go through it, so
// business rules (SLA stamping, first-response tracking, routing) live in one
// place rather than in the HTTP handlers.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService wires a store and a clock. Pass a fixed clock in tests so
// time-dependent rules (SLA, resolution time) are deterministic.
func NewService(store Store, clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: store, now: clock}
}

// Store exposes the underlying store for read-only lookups the HTTP layer needs
// (agents, teams, kb). Operations that mutate tickets go through Service methods.
func (s *Service) Store() Store { return s.store }
