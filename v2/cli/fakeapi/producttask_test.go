package fakeapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductTaskServesP1(t *testing.T) {
	wd, _ := os.Getwd() // .../v2/cli/fakeapi
	product := filepath.Clean(filepath.Join(wd, "../../products/p1-support-saas"))
	if _, err := os.Stat(filepath.Join(product, "go.mod")); err != nil {
		t.Skipf("P1 product not present at %s", product)
	}

	task, err := ProductTask(product, "feature-ticket-merge", "p1@merge-v1", "# Implement merge")
	if err != nil {
		t.Fatal(err)
	}

	// Product source + the overlay stub are served; TASK.md is the brief.
	if _, ok := task.Files["internal/support/tickets.go"]; !ok {
		t.Fatal("product source should be served")
	}
	if s, ok := task.Files["internal/support/merge.go"]; !ok || !strings.Contains(s, "not implemented") {
		t.Fatal("the overlay stub should be served")
	}
	if task.Files["TASK.md"] != "# Implement merge" {
		t.Fatalf("TASK.md should be the brief, got %q", task.Files["TASK.md"])
	}
	if len(task.Locked) != 1 || task.Locked[0] != "TASK.md" {
		t.Fatalf("TASK.md should be locked, got %v", task.Locked)
	}
	if len(task.CheckCommands) == 0 || task.CheckCommands[0] != "go" {
		t.Fatalf("check command should come from task.json, got %v", task.CheckCommands)
	}
	if len(task.Runtimes) != 1 || task.Runtimes[0].Name != "go" {
		t.Fatalf("go runtime should be detected, got %v", task.Runtimes)
	}

	// The answer key is never served.
	for p := range task.Files {
		if strings.Contains(p, "tasks/") || strings.Contains(p, "acceptance") || strings.Contains(p, "solution") {
			t.Fatalf("answer key leaked into the served task: %s", p)
		}
	}
}
