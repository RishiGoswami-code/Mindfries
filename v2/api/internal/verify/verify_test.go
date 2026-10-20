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
	return product, func(id string) string { return filepath.Join(product, "_tasks", id) }
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

// p2 returns the P2 product path and a task-dir resolver.
func p2(t *testing.T) (product string, taskDir func(id string) string) {
	t.Helper()
	wd, _ := os.Getwd()
	product = filepath.Clean(filepath.Join(wd, "../../../products/p2-rag-platform"))
	if _, err := os.Stat(filepath.Join(product, "go.mod")); err != nil {
		t.Fatalf("P2 product not found at %s", product)
	}
	return product, func(id string) string { return filepath.Join(product, "_tasks", id) }
}

func TestP2DebugTaskBugFailsAndFixPasses(t *testing.T) {
	product, task := p2(t)
	d := task("debug-retrieval-order")
	check := []string{"go", "test", "./internal/rag/"}

	bug, err := Run(Spec{ProductDir: product, Overlays: []string{filepath.Join(d, "overlay")}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if bug.Passed {
		t.Fatal("P2 retrieval-order bug should fail the suite")
	}
	fix, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(d, "overlay"), filepath.Join(d, "solution"),
	}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if !fix.Passed {
		t.Fatalf("P2 retrieval-order fix should green the suite:\n%s", fix.Output)
	}
}

func TestP2FeatureTaskStubFailsAndSolutionPasses(t *testing.T) {
	product, task := p2(t)
	f := task("feature-recall-at-k")
	check := []string{"go", "test", "./internal/rag/"}

	stub, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(f, "overlay"), filepath.Join(f, "acceptance"),
	}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if stub.Passed {
		t.Fatal("P2 RecallAtK stub should fail the acceptance tests")
	}
	sol, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(f, "overlay"), filepath.Join(f, "solution"), filepath.Join(f, "acceptance"),
	}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if !sol.Passed {
		t.Fatalf("P2 RecallAtK solution should pass acceptance:\n%s", sol.Output)
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

// p3 returns the P3 (Python) product path and a task-dir resolver.
func p3(t *testing.T) (product string, taskDir func(id string) string) {
	t.Helper()
	wd, _ := os.Getwd()
	product = filepath.Clean(filepath.Join(wd, "../../../products/p3-quant-platform"))
	if _, err := os.Stat(filepath.Join(product, "quant", "__init__.py")); err != nil {
		t.Fatalf("P3 product not found at %s", product)
	}
	return product, func(id string) string { return filepath.Join(product, "_tasks", id) }
}

var pyCheck = []string{"python3", "-m", "unittest", "discover", "-s", "tests", "-t", "."}

func TestP3DebugTaskBugFailsAndFixPasses(t *testing.T) {
	product, task := p3(t)
	d := task("debug-drawdown")

	bug, err := Run(Spec{ProductDir: product, Overlays: []string{filepath.Join(d, "overlay")}, Check: pyCheck})
	if err != nil {
		t.Fatal(err)
	}
	if bug.Passed {
		t.Fatal("P3 drawdown bug should fail the suite")
	}
	fix, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(d, "overlay"), filepath.Join(d, "solution"),
	}, Check: pyCheck})
	if err != nil {
		t.Fatal(err)
	}
	if !fix.Passed {
		t.Fatalf("P3 drawdown fix should green the suite:\n%s", fix.Output)
	}
}

func TestP3FeatureTaskStubFailsAndSolutionPasses(t *testing.T) {
	product, task := p3(t)
	f := task("feature-sortino")

	stub, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(f, "overlay"), filepath.Join(f, "acceptance"),
	}, Check: pyCheck})
	if err != nil {
		t.Fatal(err)
	}
	if stub.Passed {
		t.Fatal("P3 sortino stub should fail the acceptance tests")
	}
	sol, err := Run(Spec{ProductDir: product, Overlays: []string{
		filepath.Join(f, "overlay"), filepath.Join(f, "solution"), filepath.Join(f, "acceptance"),
	}, Check: pyCheck})
	if err != nil {
		t.Fatal(err)
	}
	if !sol.Passed {
		t.Fatalf("P3 sortino solution should pass acceptance:\n%s", sol.Output)
	}
}
