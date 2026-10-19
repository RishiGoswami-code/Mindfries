package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/assistant"
	"github.com/mindfries/v2/cli/internal/ledger"
	"github.com/mindfries/v2/cli/internal/ui"
)

type attributed struct {
	Path   string `json:"path"`
	Origin string `json:"origin"`
	Lines  int    `json:"lines"`
	EditID string `json:"edit_id"`
}

func (e *env) attributions() []attributed {
	var out []attributed
	for _, ev := range e.ofType("change.attributed") {
		var a attributed
		json.Unmarshal(ev.Data, &a)
		out = append(out, a)
	}
	return out
}

func TestAttributionInAMixedSession(t *testing.T) {
	e := newEnv(t, fakeapi.SessionOptions{})
	wait := e.run(e.config())

	// 1. mindfries agent applies an edit the candidate approved.
	x := &assistant.Executor{
		Root: e.ws, Locked: map[string]bool{"TASK.md": true},
		UI: ui.New(strings.NewReader("y\n"), io.Discard, false), Out: io.Discard,
		Record: func(ed api.AppliedEdit) error { return ledger.Append(e.stateDir, ed) },
	}
	input, _ := json.Marshal(api.ProposeEditInput{Path: "helper.py", Create: true, Edits: []api.TextEdit{{NewText: strings.Repeat("value = compute()\n", 40)}}})
	res := x.Execute(context.Background(), api.ToolCall{CallID: "c1", Name: api.ToolProposeEdit, Input: input})
	if res.AppliedEdit == nil {
		t.Fatalf("edit not applied: %+v", res)
	}
	e.eventually("agent edit attributed", func() bool { return len(e.attributions()) == 1 })

	// 2. The candidate types, a few lines at a time.
	body := ""
	for i := 0; i < 6; i++ {
		body += fmt.Sprintf("total_%d = %d\n", i, i)
		e.write("solution.py", body)
		time.Sleep(60 * time.Millisecond)
	}
	e.eventually("typing attributed", func() bool {
		for _, a := range e.attributions() {
			if a.Path == "solution.py" && strings.Contains(e.lastBody("solution.py"), "total_5") {
				return true
			}
		}
		return false
	})
	time.Sleep(100 * time.Millisecond)

	// 3. A large block pasted in at once, from neither.
	e.write("pasted.py", strings.Repeat("x = 1  # from somewhere\n", 60))
	e.eventually("paste attributed", func() bool {
		for _, a := range e.attributions() {
			if a.Path == "pasted.py" {
				return true
			}
		}
		return false
	})
	RequestStop(e.stateDir, ReasonStop)
	wait()

	for _, a := range e.attributions() {
		var want string
		switch a.Path {
		case "helper.py":
			want = OriginAssistant
			if a.EditID != res.AppliedEdit.EditID || a.Lines != 41 {
				t.Errorf("assistant edit: %+v", a)
			}
		case "solution.py":
			want = OriginCandidate
		case "pasted.py":
			want = OriginUnknown
		}
		if a.Origin != want {
			t.Errorf("%s attributed to %s, want %s", a.Path, a.Origin, want)
		}
	}
	// The assistant's edits never count towards a burst.
	for _, b := range e.ofType("file.burst") {
		if strings.Contains(string(b.Data), "helper.py") {
			t.Error("the assistant's edit was flagged as a burst")
		}
	}
}

// lastBody is a helper for the typing loop's exit condition.
func (e *env) lastBody(name string) string {
	b, _ := readFile(e.ws, name)
	return b
}
