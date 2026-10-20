package delivery

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/taskgen"
	"github.com/mindfries/v2/api/internal/verify"
)

func p1Dir(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd() // .../v2/api/internal/delivery
	d := filepath.Clean(filepath.Join(wd, "../../../products/p1-support-saas"))
	if _, err := os.Stat(filepath.Join(d, "go.mod")); err != nil {
		t.Fatalf("P1 not found at %s", d)
	}
	return d
}

func TestAssembleFeatureWorkspace(t *testing.T) {
	product := p1Dir(t)
	ws, err := Assemble(product, "feature-ticket-merge", "# Do the merge task", "task_abc")
	if err != nil {
		t.Fatal(err)
	}

	// Product source is present.
	if _, ok := ws.Files["internal/support/tickets.go"]; !ok {
		t.Fatal("product source should be delivered")
	}
	// The overlay stub is applied.
	merge, ok := ws.Files["internal/support/merge.go"]
	if !ok || !contains(merge, "not implemented") {
		t.Fatal("the overlay merge stub should be delivered")
	}
	// TASK.md is the brief and is locked.
	if ws.Files["TASK.md"] != "# Do the merge task" {
		t.Fatalf("TASK.md should be the brief, got %q", ws.Files["TASK.md"])
	}
	if len(ws.Locked) != 1 || ws.Locked[0] != "TASK.md" {
		t.Fatalf("TASK.md should be locked, got %v", ws.Locked)
	}
	// Metadata.
	if len(ws.Check) == 0 || ws.Check[0] != "go" {
		t.Fatalf("check command should come from task.json, got %v", ws.Check)
	}
	if len(ws.Runtimes) != 1 || ws.Runtimes[0] != "go" {
		t.Fatalf("runtimes should detect go, got %v", ws.Runtimes)
	}

	// The answer key is NEVER delivered.
	for p := range ws.Files {
		if contains(p, "tasks/") {
			t.Fatalf("task scaffolding leaked into the workspace: %s", p)
		}
		if contains(p, "acceptance") || contains(p, "solution") {
			t.Fatalf("answer key leaked into the workspace: %s", p)
		}
	}
	if contains(ws.Files["internal/support/merge.go"], "StatusClosed") {
		t.Fatal("the reference solution must not be in the delivered stub")
	}
}

// The delivered workspace must be exactly the thing that fails at the stub and
// passes once solved — proving selection → delivery → verification all line up.
func TestDeliveredWorkspaceIsSolvableAndChecked(t *testing.T) {
	product := p1Dir(t)
	taskID := "feature-ticket-merge"
	ws, err := Assemble(product, taskID, "# merge", "task_x")
	if err != nil {
		t.Fatal(err)
	}

	// Materialize what the candidate receives.
	work := t.TempDir()
	if err := ws.WriteTo(work); err != nil {
		t.Fatal(err)
	}

	acceptance := filepath.Join(product, "_tasks", taskID, "acceptance")
	solution := filepath.Join(product, "_tasks", taskID, "solution")
	check := ws.Check

	// As delivered (stub) + hidden acceptance tests: must fail.
	stub, err := verify.Run(verify.Spec{ProductDir: work, Overlays: []string{acceptance}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if stub.Passed {
		t.Fatal("the delivered stub should fail the acceptance tests")
	}

	// Candidate applies the reference solution: must pass.
	solved, err := verify.Run(verify.Spec{ProductDir: work, Overlays: []string{solution, acceptance}, Check: check})
	if err != nil {
		t.Fatal(err)
	}
	if !solved.Passed {
		t.Fatalf("the delivered workspace should pass once solved:\n%s", solved.Output)
	}
}

// Selector → taskgen → delivery, wired end to end in one module.
func TestFromResolvedPipeline(t *testing.T) {
	product := p1Dir(t)
	bank, err := taskgen.LoadBank(product, "p1-support-saas")
	if err != nil {
		t.Fatal(err)
	}
	g := taskgen.New(bank, taskgen.NewMemCache(), nil, 3)
	rt, err := g.Resolve(context.Background(), "p1-support-saas", core.Backend, "implement ticket merge for duplicates")
	if err != nil {
		t.Fatal(err)
	}
	ws, err := FromResolved(product, rt)
	if err != nil {
		t.Fatal(err)
	}
	if ws.TaskVersion != rt.TaskVersion || ws.Files["TASK.md"] != rt.Brief {
		t.Fatal("workspace should carry the resolved task's version and brief")
	}
	if _, ok := ws.Files["internal/support/merge.go"]; !ok {
		t.Fatal("the resolved merge task's overlay should be delivered")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// A frontend task on P1 must require node, not go — even though P1 ships a
// go.mod for its backend. (Regression: the preflight demanded Go for a React
// task and blocked the candidate.)
func TestFrontendTaskRequiresNodeNotGo(t *testing.T) {
	ws, err := Assemble(p1Dir(t), "debug-priority-order", "# Fix the sort", "task_fe")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Runtimes) != 1 || ws.Runtimes[0] != "node" {
		t.Fatalf("frontend task runtimes = %v, want [node]", ws.Runtimes)
	}
}
