package support

// Metrics is the support analytics snapshot — the Data-Analyst subsystem's
// output and what the reporting module/dashboard renders.
type Metrics struct {
	Total      int              `json:"total"`
	ByStatus   map[Status]int   `json:"by_status"`
	ByPriority map[Priority]int `json:"by_priority"`
	Resolved   int              `json:"resolved"`

	// AvgResolutionSeconds averages ResolvedAt-CreatedAt over resolved tickets.
	AvgResolutionSeconds float64 `json:"avg_resolution_seconds"`
	// FirstResponseSLACompliance is the fraction of tickets (with a policy)
	// whose first response met the SLA. 0–1.
	FirstResponseSLACompliance float64 `json:"first_response_sla_compliance"`
	// ResolutionSLACompliance is the fraction of resolved tickets that met the
	// resolution SLA. 0–1.
	ResolutionSLACompliance float64 `json:"resolution_sla_compliance"`
	// AvgCSAT averages satisfaction over rated tickets; 0 when none rated.
	AvgCSAT float64 `json:"avg_csat"`
}

// Metrics computes the snapshot over the tickets matching filter.
func (s *Service) Metrics(filter TicketFilter) (Metrics, error) {
	tickets, err := s.store.ListTickets(filter)
	if err != nil {
		return Metrics{}, err
	}

	m := Metrics{
		Total:      len(tickets),
		ByStatus:   map[Status]int{},
		ByPriority: map[Priority]int{},
	}

	var resSecondsSum float64
	var frWithPolicy, frMet int
	var resMet int
	var csatSum, csatN int

	for _, t := range tickets {
		m.ByStatus[t.Status]++
		m.ByPriority[t.Priority]++

		if t.ResolvedAt != nil {
			m.Resolved++
			resSecondsSum += t.ResolvedAt.Sub(t.CreatedAt).Seconds()
			if !s.ResolutionBreached(t) {
				resMet++
			}
		}

		if _, ok := s.policyFor(t.Priority); ok {
			frWithPolicy++
			if !s.FirstResponseBreached(t) {
				frMet++
			}
		}

		if t.Satisfaction > 0 {
			csatSum += t.Satisfaction
			csatN++
		}
	}

	if m.Resolved > 0 {
		m.AvgResolutionSeconds = resSecondsSum / float64(m.Resolved)
		m.ResolutionSLACompliance = float64(resMet) / float64(m.Resolved)
	}
	if frWithPolicy > 0 {
		m.FirstResponseSLACompliance = float64(frMet) / float64(frWithPolicy)
	}
	if csatN > 0 {
		m.AvgCSAT = float64(csatSum) / float64(csatN)
	}
	return m, nil
}
