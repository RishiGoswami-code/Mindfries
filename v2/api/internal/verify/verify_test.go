package verify

import (
	"os"
	"path/filepath"
	"testing"
)

// p1 returns the absolute path to the P1 product and one of its task dirs.
func p1(t *testing.T) (product string, taskDir func(id string) string) {
	t.Helper()
	wd, err := os.Getwd() // .../v2/api/internal/verify
	if err != nil {
		t.Fatal(err)
	}
	product = filepath.Clean(filepath.Join(wd, "../../../products/p1-support-saas"))
	if _, err := os.Stat(filepath.Join(product, "go.mod")); err != nil {
		t.Fatalf("P1 product not found at %s: %v", product, err)
	}
	return product, func(id string) string { return filepath.Join(product, "tasks", id) }
}

// The golden product must be green on its own — the correctness contract.
func TestGoldenProductPasses(t *testing.T) {
	product, _ := p1(t)
	r, err := Run(Spec{ProductDir: product, Check: []string{"go", "test", "./..."}})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatalf("golden product suite should pass:\n%s", r.Output)
	}
}

func TestDebugTaskBugFailsAndFixPasses(t *testing.T) {
	product, task := p1(t)
	d := task("debug-auto-assign")
	check := []string{"go", "test", "./internal/support/"}

	// The planted bug: suite fails.
	bug, err := Run(Spec{ProductDir: product, Overlays: []string{filepath.Join(d, "overlay")}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if bug.Passed {
		t.Fatal("debug overlay should make the suite fail, but it passed")
	}

	// The reference fix: suite greens.
	fix, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(d, "overlay"), filepath.Join(d, "solution"),
	}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if !fix.Passed {
		t.Fatalf("debug solution should green the suite:\n%s", fix.Output)
	}
}

func TestFeatureTaskStubFailsAndSolutionPasses(t *testing.T) {
	product, task := p1(t)
	f := task("feature-ticket-merge")
	check := []string{"go", "test", "./internal/support/"}

	// Stub + hidden acceptance tests: fails (feature not implemented).
	stub, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(f, "overlay"), filepath.Join(f, "acceptance"),
	}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if stub.Passed {
		t.Fatal("feature stub should fail the acceptance tests, but it passed")
	}

	// Reference solution + acceptance tests: passes (feature is solvable).
	sol, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(f, "overlay"), filepath.Join(f, "solution"), filepath.Join(f, "acceptance"),
	}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if !sol.Passed {
		t.Fatalf("feature solution should pass acceptance:\n%s", sol.Output)
	}
}
