package manifest

import (
	"testing"

	"github.com/mindfries/v2/api/internal/core"
)

func TestValidateAcceptsAGoodManifest(t *testing.T) {
	m := Manifest{
		Tags: []string{"support", "tickets"},
		Roles: map[core.Role]RoleCoverage{
			core.Backend: {Subsystems: []string{"api/"}, Modes: []core.Mode{core.Implementation, core.Debugging}},
		},
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if !m.Covers(core.Backend) || m.Covers(core.Quant) {
		t.Fatal("Covers is wrong")
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	raw := `{"roles":{"backend":{"subsystems":[],"modes":["nonsense"]},"wizard":{"subsystems":["x"],"modes":["implementation"]}}}`
	_, err := Parse([]byte(raw))
	if err == nil {
		t.Fatal("expected validation errors")
	}
	// Should catch: empty subsystems, unknown mode, unknown role — all three.
	for _, want := range []string{"subsystem", "unknown mode", "unknown role"} {
		if !contains(err.Error(), want) {
			t.Fatalf("missing %q in: %v", want, err)
		}
	}
}

func TestParseRejectsEmptyRoles(t *testing.T) {
	if _, err := Parse([]byte(`{"roles":{}}`)); err == nil {
		t.Fatal("a product with no roles must be rejected")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
