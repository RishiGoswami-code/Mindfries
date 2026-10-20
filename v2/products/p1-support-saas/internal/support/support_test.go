package support

import (
	"testing"
	"time"
)

// clk returns a controllable clock starting at a fixed instant.
func clk(start time.Time) (func() time.Time, *time.Time) {
	now := start
	return func() time.Time { return now }, &now
}

var t0 = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

func newSeeded(clock func() time.Time) *Service {
	store := NewMemStore()
	store.Seed(
		[]Agent{
			{ID: "A1", Name: "Ada", TeamID: "T1", Active: true},
			{ID: "A2", Name: "Bo", TeamID: "T1", Active: true},
			{ID: "A3", Name: "Cy", TeamID: "T2", Active: false},
		},
		[]Team{{ID: "T1", Name: "Tier 1"}, {ID: "T2", Name: "Tier 2"}},
		[]SLAPolicy{
			{ID: "P-urgent", Priority: PriorityUrgent, FirstResponseTime: time.Hour, ResolutionTime: 4 * time.Hour},
			{ID: "P-normal", Priority: PriorityNormal, FirstResponseTime: 8 * time.Hour, ResolutionTime: 48 * time.Hour},
		},
		[]KBArticle{
			{ID: "KB1", Title: "Reset your password", Body: "Use the forgot password link to reset login access.", Tags: []string{"password", "login"}},
			{ID: "KB2", Title: "Billing and invoices", Body: "Download invoices from the billing page.", Tags: []string{"billing", "invoice"}},
			{ID: "KB3", Title: "Two-factor authentication", Body: "Enable 2FA from security settings.", Tags: []string{"security", "login"}},
		},
	)
	return NewService(store, clock)
}

func TestCreateAndReplyLifecycle(t *testing.T) {
	clock, now := clk(t0)
	s := newSeeded(clock)

	tk, err := s.Create("Cannot log in", "user@acme.test", PriorityUrgent, "I can't access my account")
	if err != nil {
		t.Fatal(err)
	}
	if tk.Status != StatusOpen || tk.Priority != PriorityUrgent || len(tk.Messages) != 1 {
		t.Fatalf("unexpected new ticket: %+v", tk)
	}

	// Agent replies 30 min later -> stamps first response.
	*now = t0.Add(30 * time.Minute)
	tk, err = s.Reply(tk.ID, AuthorAgent, "A1", "Looking into it now.")
	if err != nil {
		t.Fatal(err)
	}
	if tk.FirstResponse == nil || !tk.FirstResponse.Equal(t0.Add(30*time.Minute)) {
		t.Fatalf("first response not stamped: %+v", tk.FirstResponse)
	}

	// Resolve, then a customer reply reopens.
	tk, _ = s.SetStatus(tk.ID, StatusResolved)
	if tk.ResolvedAt == nil {
		t.Fatal("resolve should stamp ResolvedAt")
	}
	tk, _ = s.Reply(tk.ID, AuthorCustomer, "", "Still broken!")
	if tk.Status != StatusOpen || tk.ResolvedAt != nil {
		t.Fatalf("customer reply should reopen, got status=%s resolvedAt=%v", tk.Status, tk.ResolvedAt)
	}
}

func TestReplyRequiresBodyAndRateGuards(t *testing.T) {
	clock, _ := clk(t0)
	s := newSeeded(clock)
	tk, _ := s.Create("x", "u@t.test", PriorityNormal, "hi")

	if _, err := s.Reply(tk.ID, AuthorAgent, "A1", "   "); err == nil {
		t.Fatal("empty reply must error")
	}
	if _, err := s.Rate(tk.ID, 5); err == nil {
		t.Fatal("rating an open ticket must error")
	}
	s.SetStatus(tk.ID, StatusResolved)
	if _, err := s.Rate(tk.ID, 6); err == nil {
		t.Fatal("out-of-range score must error")
	}
	if tk2, err := s.Rate(tk.ID, 4); err != nil || tk2.Satisfaction != 4 {
		t.Fatalf("valid rating failed: %v %+v", err, tk2)
	}
}

func TestSLABreachDetection(t *testing.T) {
	clock, now := clk(t0)
	s := newSeeded(clock)
	tk, _ := s.Create("Urgent outage", "u@t.test", PriorityUrgent, "down")

	// 30 min in, within the 1h first-response SLA, no reply yet -> not breached.
	*now = t0.Add(30 * time.Minute)
	if s.FirstResponseBreached(tk) {
		t.Fatal("should not be breached within SLA")
	}
	// 90 min in, still no reply -> first-response breached.
	*now = t0.Add(90 * time.Minute)
	if !s.FirstResponseBreached(tk) {
		t.Fatal("should be breached past SLA with no reply")
	}
	// 5h in, unresolved -> resolution breached (target 4h).
	*now = t0.Add(5 * time.Hour)
	if !s.ResolutionBreached(tk) {
		t.Fatal("should be resolution-breached past 4h")
	}

	breached, _ := s.BreachedTickets()
	if len(breached) != 1 {
		t.Fatalf("expected 1 breached ticket, got %d", len(breached))
	}
}

func TestAutoAssignBalancesLoad(t *testing.T) {
	clock, _ := clk(t0)
	s := newSeeded(clock)

	// First ticket -> A1 (tie broken by id). Second -> A2 (A1 now loaded).
	tk1, _ := s.Create("one", "u@t.test", PriorityNormal, "")
	tk1, err := s.AutoAssign(tk1.ID)
	if err != nil || tk1.AssigneeID != "A1" {
		t.Fatalf("first auto-assign should go to A1, got %q (%v)", tk1.AssigneeID, err)
	}
	tk2, _ := s.Create("two", "u@t.test", PriorityNormal, "")
	tk2, _ = s.AutoAssign(tk2.ID)
	if tk2.AssigneeID != "A2" {
		t.Fatalf("second auto-assign should balance to A2, got %q", tk2.AssigneeID)
	}
	// Inactive agent A3 is never chosen.
	tk3, _ := s.Create("three", "u@t.test", PriorityNormal, "")
	tk3, _ = s.AutoAssign(tk3.ID)
	if tk3.AssigneeID == "A3" {
		t.Fatal("inactive agent must not be assigned")
	}
}

func TestSuggestRanksByOverlap(t *testing.T) {
	clock, _ := clk(t0)
	s := newSeeded(clock)
	tk, _ := s.Create("Password reset help", "u@t.test", PriorityNormal, "I need to reset my login password")

	sug, err := s.Suggest(tk.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(sug) == 0 || sug[0].Article.ID != "KB1" {
		t.Fatalf("expected KB1 (password/login) first, got %+v", sug)
	}
	// Billing article shares no keywords -> should not appear.
	for _, x := range sug {
		if x.Article.ID == "KB2" {
			t.Fatal("unrelated billing article should not be suggested")
		}
	}
}

func TestMetrics(t *testing.T) {
	clock, now := clk(t0)
	s := newSeeded(clock)

	// Ticket resolved within SLA, rated 5.
	a, _ := s.Create("a", "u@t.test", PriorityNormal, "hi")
	*now = t0.Add(1 * time.Hour)
	s.Reply(a.ID, AuthorAgent, "A1", "on it")
	*now = t0.Add(2 * time.Hour)
	s.SetStatus(a.ID, StatusResolved)
	s.Rate(a.ID, 5)

	// Ticket left open and breaching.
	s.Create("b", "u@t.test", PriorityUrgent, "down")
	*now = t0.Add(10 * time.Hour)

	m, err := s.Metrics(TicketFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Total != 2 || m.Resolved != 1 {
		t.Fatalf("counts wrong: %+v", m)
	}
	if m.AvgCSAT != 5 {
		t.Fatalf("avg CSAT should be 5, got %v", m.AvgCSAT)
	}
	if m.ResolutionSLACompliance != 1 {
		t.Fatalf("the one resolved ticket met SLA, want 1.0, got %v", m.ResolutionSLACompliance)
	}
	// One of two tickets (the urgent one) is first-response breached.
	if m.FirstResponseSLACompliance != 0.5 {
		t.Fatalf("first-response compliance want 0.5, got %v", m.FirstResponseSLACompliance)
	}
}
