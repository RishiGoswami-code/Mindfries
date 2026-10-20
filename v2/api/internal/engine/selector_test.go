package engine

import (
	"testing"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/manifest"
)

func roleCov(subsys ...string) manifest.RoleCoverage {
	return manifest.RoleCoverage{Subsystems: subsys, Modes: []core.Mode{core.Implementation}}
}

// Two products both serve Backend; the JD should disambiguate by domain tags.
func products() []Product {
	p1 := Product{Slug: "p1-support-saas", Name: "Support SaaS", Domain: "customer support saas",
		Manifest: manifest.Manifest{
			Tags:  []string{"support", "tickets", "helpdesk", "saas"},
			Roles: map[core.Role]manifest.RoleCoverage{core.Backend: roleCov("api/"), core.Frontend: roleCov("web/")},
		}}
	p4 := Product{Slug: "p4-fintech-ledger", Name: "Fintech Ledger", Domain: "payments ledger fintech",
		Manifest: manifest.Manifest{
			Tags:  []string{"payments", "ledger", "settlement", "fintech"},
			Roles: map[core.Role]manifest.RoleCoverage{core.Backend: roleCov("ledger/")},
		}}
	p3 := Product{Slug: "p3-quant", Name: "Quant Platform", Domain: "quantitative research",
		Manifest: manifest.Manifest{
			Tags:  []string{"backtest", "portfolio", "alpha", "timeseries"},
			Roles: map[core.Role]manifest.RoleCoverage{core.Quant: roleCov("research/")},
		}}
	return []Product{p1, p4, p3}
}

func TestSelectDisambiguatesByJD(t *testing.T) {
	c, err := Select(core.Backend, "Backend engineer for our payments ledger and settlement system", products())
	if err != nil {
		t.Fatal(err)
	}
	if c.Product.Slug != "p4-fintech-ledger" {
		t.Fatalf("payments JD should pick the ledger product, got %q (%s)", c.Product.Slug, c.Why)
	}

	c, err = Select(core.Backend, "Backend engineer on our customer support helpdesk SaaS", products())
	if err != nil {
		t.Fatal(err)
	}
	if c.Product.Slug != "p1-support-saas" {
		t.Fatalf("support JD should pick the support product, got %q (%s)", c.Product.Slug, c.Why)
	}
}

func TestSelectRoleOnlyStillPicks(t *testing.T) {
	// A JD with no matching tags still resolves (role-only pick), stably.
	c, err := Select(core.Quant, "We want someone great", products())
	if err != nil {
		t.Fatal(err)
	}
	if c.Product.Slug != "p3-quant" {
		t.Fatalf("quant role should resolve to the quant product, got %q", c.Product.Slug)
	}
}

func TestSelectNoCoverageErrors(t *testing.T) {
	// No product covers FDE in this set -> loud error, not a bad match.
	if _, err := Select(core.FDE, "anything", products()); err == nil {
		t.Fatal("expected an error when no product covers the role")
	}
}
