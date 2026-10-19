package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/api"
)

func logKinds(h *harness) map[string][]string {
	out := map[string][]string{}
	for _, l := range h.srv.AssistantLog(h.srv.OnlySessionID()) {
		out[l.Kind] = append(out[l.Kind], l.Content)
	}
	return out
}

func TestAgentAppliesAnApprovedEdit(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask()})
	exit, out := h.run("create notes.md with hello\ny\n/budget\n/exit\n", "agent")
	if exit != 0 {
		t.Fatalf("agent: %d\n%s", exit, out)
	}
	for _, want := range []string{"can propose code changes", "Budget: ", "+++ notes.md", "+hello", "Applied to notes.md", "notes.md is updated", "tokens left"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "notes.md")); string(b) != "hello\n" {
		t.Fatalf("notes.md = %q", b)
	}
	if res := logKinds(h)["tool_result"]; len(res) != 1 || !strings.Contains(res[0], `"applied_edit"`) {
		t.Fatalf("logged results: %v", res)
	}
}

func TestAgentRejectedEditChangesNothing(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask()})
	exit, out := h.run("create notes.md with hello\nn\n/exit\n", "agent")
	if exit != 0 || !strings.Contains(out, "won't make that change") {
		t.Fatalf("agent: %d\n%s", exit, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); !os.IsNotExist(err) {
		t.Fatal("a rejected edit was written")
	}
	if res := logKinds(h)["tool_result"]; len(res) != 1 || !strings.Contains(res[0], `"rejected"`) {
		t.Fatalf("logged results: %v", res)
	}
}

func TestAgentRunsAnApprovedCommand(t *testing.T) {
	h := newHarness(t)
	h.started(fakeapi.SessionOptions{Task: quickTask()})
	exit, out := h.run("run git --version\ny\n/exit\n", "agent")
	if exit != 0 || !strings.Contains(out, "wants to run") || !strings.Contains(out, "git version") || !strings.Contains(out, "exit code 0") {
		t.Fatalf("agent: %d\n%s", exit, out)
	}
}

func TestAgentExplainOnly(t *testing.T) {
	h := newHarness(t)
	dir := h.started(fakeapi.SessionOptions{Task: quickTask(), Policy: api.Policy{AssistantMode: api.ModeExplainOnly}})
	exit, out := h.run("create notes.md with hello\n/exit\n", "agent")
	if exit != 0 || !strings.Contains(out, "explain only") || !strings.Contains(out, "can't write code") || strings.Contains(out, "Apply this change") {
		t.Fatalf("agent: %d\n%s", exit, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); !os.IsNotExist(err) {
		t.Fatal("explain-only mode wrote a file")
	}
}

func TestAgentOff(t *testing.T) {
	h := newHarness(t)
	h.started(fakeapi.SessionOptions{Task: quickTask(), Policy: api.Policy{AssistantMode: api.ModeOff}})
	if exit, out := h.run("hello\n", "agent"); exit != 0 || !strings.Contains(out, "off for this assessment") {
		t.Fatalf("agent: %d\n%s", exit, out)
	}
}

func TestAgentResearchAndMultiline(t *testing.T) {
	h := newHarness(t)
	h.started(fakeapi.SessionOptions{Task: quickTask()})
	exit, out := h.run("research https://docs.python.org/3/library/collections.html\nresearch https://example.org/x\nHow do I \\\nstart?\n/exit\n", "agent")
	if exit != 0 || !strings.Contains(out, "[source]") || !strings.Contains(out, "[research refused: example.org") || !strings.Contains(out, "What have you tried") {
		t.Fatalf("agent: %d\n%s", exit, out)
	}
	prompts := logKinds(h)["prompt"]
	if len(prompts) != 3 || prompts[2] != "How do I \nstart?" {
		t.Fatalf("prompts: %q", prompts)
	}
}

func TestAgentNeedsASession(t *testing.T) {
	h := newHarness(t)
	if exit, out := h.run("", "agent"); exit == 0 || !strings.Contains(out, "no active assessment") {
		t.Fatalf("%d\n%s", exit, out)
	}
}
