package taskgen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindfries/v2/api/internal/core"
)

func demoBank() []BankTask {
	return []BankTask{
		{ID: "feature-ticket-merge", ProductSlug: "p1", Role: core.Backend, Kind: core.Feature,
			Title: "Implement ticket merge", Tags: []string{"merge", "duplicate", "tickets"}, Brief: "Implement merge."},
		{ID: "debug-auto-assign", ProductSlug: "p1", Role: core.Backend, Kind: core.Debug,
			Title: "Fix auto-assign", Tags: []string{"assignment", "routing", "load", "balancing"}, Brief: "Fix routing."},
		{ID: "metrics-task", ProductSlug: "p1", Role: core.DataAnalyst, Kind: core.Feature,
			Title: "Add a metric", Tags: []string{"metrics", "sla", "csat"}, Brief: "Add metric."},
	}
}

// countingProvider records how many times Adapt is called, so we can assert a
// pool is built once per JD and reused afterwards.
type countingProvider struct {
	calls int
	fail  bool
}

func (p *countingProvider) Adapt(_ context.Context, brief, jd string) (string, error) {
	p.calls++
	if p.fail {
		return "", errors.New("model unavailable")
	}
	return brief + "\n\n(tailored to this role)", nil
}

func TestResolveSelectsByJD(t *testing.T) {
	g := New(demoBank(), NewMemCache(), nil, 3)

	merge, err := g.Resolve(context.Background(), "p1", core.Backend, "We need someone to merge duplicate tickets")
	if err != nil {
		t.Fatal(err)
	}
	if merge.BankTaskID != "feature-ticket-merge" {
		t.Fatalf("merge JD should select the merge task, got %q", merge.BankTaskID)
	}

	routing, _ := g.Resolve(context.Background(), "p1", core.Backend, "fix ticket assignment load balancing in routing")
	if routing.BankTaskID != "debug-auto-assign" {
		t.Fatalf("routing JD should select the auto-assign task, got %q", routing.BankTaskID)
	}
}

func TestCacheBuildsPoolOncePerJD(t *testing.T) {
	prov := &countingProvider{}
	g := New(demoBank(), NewMemCache(), prov, 2) // 2 backend tasks -> pool of 2

	jd := "backend engineer, tickets and routing"
	first, _ := g.Resolve(context.Background(), "p1", core.Backend, jd)
	callsAfterBuild := prov.calls // one Adapt per pool entry
	if callsAfterBuild != 2 {
		t.Fatalf("pool build should adapt each of 2 variants once, got %d", callsAfterBuild)
	}

	// Subsequent calls with the same JD hit the cache: no more Adapt calls, and
	// the served variant rotates.
	second, _ := g.Resolve(context.Background(), "p1", core.Backend, jd)
	if prov.calls != callsAfterBuild {
		t.Fatalf("cached JD must not rebuild the pool (adapt calls went %d -> %d)", callsAfterBuild, prov.calls)
	}
	if first.VariantNo == second.VariantNo {
		t.Fatalf("variant should rotate across candidates, both were %d", first.VariantNo)
	}
	if first.TaskVersion == second.TaskVersion {
		t.Fatal("rotated variants should have distinct task_versions")
	}
}

func TestEditingJDRegenerates(t *testing.T) {
	prov := &countingProvider{}
	g := New(demoBank(), NewMemCache(), prov, 2)

	g.Resolve(context.Background(), "p1", core.Backend, "tickets and routing")
	callsV1 := prov.calls
	// A cosmetic change must NOT regenerate (same normalized hash).
	g.Resolve(context.Background(), "p1", core.Backend, "Tickets   and ROUTING!!")
	if prov.calls != callsV1 {
		t.Fatalf("cosmetic JD change should reuse the pool, calls %d -> %d", callsV1, prov.calls)
	}
	// A real content change regenerates.
	g.Resolve(context.Background(), "p1", core.Backend, "tickets and routing and billing analytics")
	if prov.calls <= callsV1 {
		t.Fatal("a real JD edit should rebuild the pool")
	}
}

func TestProviderFailureFallsBackToAuthoredBrief(t *testing.T) {
	prov := &countingProvider{fail: true}
	g := New(demoBank(), NewMemCache(), prov, 1)
	r, err := g.Resolve(context.Background(), "p1", core.Backend, "merge duplicate tickets")
	if err != nil {
		t.Fatal(err)
	}
	if r.Brief != "Implement merge." { // authored brief, unmodified
		t.Fatalf("a failing provider must fall back to the authored brief, got %q", r.Brief)
	}
}

func TestResolveNoBankTaskErrors(t *testing.T) {
	g := New(demoBank(), NewMemCache(), nil, 3)
	if _, err := g.Resolve(context.Background(), "p1", core.Quant, "anything"); err == nil {
		t.Fatal("expected an error when the bank has no task for the role")
	}
}

// Loads the real P1 bank from disk — proves the on-disk task format parses.
func TestLoadRealP1Bank(t *testing.T) {
	wd, _ := os.Getwd() // .../v2/api/internal/taskgen
	productDir := filepath.Clean(filepath.Join(wd, "../../../products/p1-support-saas"))
	bank, err := LoadBank(productDir, "p1-support-saas")
	if err != nil {
		t.Fatal(err)
	}
	if len(bank) < 2 {
		t.Fatalf("expected at least the 2 authored P1 tasks, got %d", len(bank))
	}
	for _, bt := range bank {
		if bt.Brief == "" || len(bt.Tags) == 0 || bt.Title == "" {
			t.Fatalf("task %q missing brief/tags/title", bt.ID)
		}
	}
}
