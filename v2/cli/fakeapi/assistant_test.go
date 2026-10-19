package fakeapi

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mindfries/v2/cli/internal/api"
)

func connect(t *testing.T, s *Server, c *api.Client, o SessionOptions) string {
	t.Helper()
	res, err := c.Exchange(context.Background(), api.ExchangeRequest{SetupCode: s.MintSetupCode(o), Device: api.Device{OS: "linux", Arch: "amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	c.Token = res.SessionToken
	return res.Session.ID
}

func drain(t *testing.T, st *api.Stream) []api.StreamEvent {
	t.Helper()
	defer st.Close()
	var out []api.StreamEvent
	for {
		ev, err := st.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
}

func text(evs []api.StreamEvent) string {
	var b strings.Builder
	for _, e := range evs {
		b.WriteString(e.Text)
	}
	return b.String()
}

func types(evs []api.StreamEvent) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Type)
	}
	return out
}

func TestExplainOnlyWritesNoCode(t *testing.T) {
	s, c := newTest(t, Config{})
	id := connect(t, s, c, SessionOptions{Policy: api.Policy{AssistantMode: api.ModeExplainOnly}})
	st, err := c.StartTurn(context.Background(), api.AssistantTurnRequest{Message: "create solution.py with print(1)"})
	if err != nil {
		t.Fatal(err)
	}
	evs := drain(t, st)
	for _, e := range evs {
		if e.Type == api.StreamToolCall {
			t.Fatalf("explain-only turn produced a tool call: %+v", e.Call)
		}
	}
	if last := evs[len(evs)-1]; last.Type != api.StreamTurnCompleted {
		t.Fatalf("ended with %s", last.Type)
	}
	if !strings.Contains(text(evs), "can't write code") {
		t.Fatalf("reply: %q", text(evs))
	}
	var refused bool
	for _, l := range s.AssistantLog(id) {
		refused = refused || l.Kind == "refused"
	}
	if !refused {
		t.Fatal("the refusal isn't in the evidence log")
	}
	// And edits can't be smuggled in as tool results.
	_, err = c.SendToolResults(context.Background(), evs[0].TurnID, api.ToolResults{Results: []api.ToolResult{{CallID: "x", Status: api.ToolOK, AppliedEdit: &api.AppliedEdit{Path: "solution.py"}}}})
	if !api.HasCode(err, api.CodeTurnNotWaiting) {
		t.Fatalf("unsolicited edit accepted: %v", err)
	}
}

func TestBudgetExhausted(t *testing.T) {
	s, c := newTest(t, Config{})
	connect(t, s, c, SessionOptions{Policy: api.Policy{AssistantMode: api.ModeCodeAllowed, AssistantBudget: &api.Budget{TokensTotal: 30, TokensRemaining: 30}}})
	st, err := c.StartTurn(context.Background(), api.AssistantTurnRequest{Message: "How should I approach a sliding window rate limiter for this task?"})
	if err != nil {
		t.Fatal(err)
	}
	drain(t, st)
	_, err = c.StartTurn(context.Background(), api.AssistantTurnRequest{Message: "and then?"})
	p, ok := api.AsProblem(err)
	if !ok || p.Code != api.CodeBudgetExhausted || p.Status != 429 || p.Title == "" {
		t.Fatalf("over budget: %v", err)
	}
}

func TestAssistantOff(t *testing.T) {
	s, c := newTest(t, Config{})
	connect(t, s, c, SessionOptions{Policy: api.Policy{AssistantMode: api.ModeOff}})
	if _, err := c.StartTurn(context.Background(), api.AssistantTurnRequest{Message: "hi"}); !api.HasCode(err, api.CodeAssistantOff) {
		t.Fatalf("off: %v", err)
	}
}

func TestToolRoundTripAndLog(t *testing.T) {
	s, c := newTest(t, Config{})
	id := connect(t, s, c, SessionOptions{})
	ctx := context.Background()
	st, err := c.StartTurn(ctx, api.AssistantTurnRequest{Message: "create notes.md with hello"})
	if err != nil {
		t.Fatal(err)
	}
	evs := drain(t, st)
	n := len(evs)
	if evs[0].Type != api.StreamTurnStarted || evs[n-2].Type != api.StreamToolCall || evs[n-1].Type != api.StreamCallsPending {
		t.Fatalf("events: %v", types(evs))
	}
	turnID, callID := evs[0].TurnID, evs[n-2].Call.CallID
	st, err = c.SendToolResults(ctx, turnID, api.ToolResults{Results: []api.ToolResult{{CallID: callID, Status: api.ToolOK, AppliedEdit: &api.AppliedEdit{EditID: "e1", Path: "notes.md", AfterSHA256: strings.Repeat("a", 64)}}}})
	if err != nil {
		t.Fatal(err)
	}
	evs = drain(t, st)
	if evs[len(evs)-1].Type != api.StreamTurnCompleted || !strings.Contains(text(evs), "notes.md is updated") {
		t.Fatalf("continuation: %v %q", types(evs), text(evs))
	}
	if _, err := c.SendToolResults(ctx, turnID, api.ToolResults{Results: []api.ToolResult{{CallID: callID, Status: api.ToolOK}}}); !api.HasCode(err, api.CodeTurnNotWaiting) {
		t.Fatalf("results accepted twice: %v", err)
	}
	kinds := map[string]bool{}
	for _, l := range s.AssistantLog(id) {
		kinds[l.Kind] = true
	}
	for _, k := range []string{"prompt", "reply", "tool_call", "tool_result"} {
		if !kinds[k] {
			t.Errorf("no %s in the evidence log", k)
		}
	}
}

func TestResearchDomains(t *testing.T) {
	s, c := newTest(t, Config{})
	connect(t, s, c, SessionOptions{Policy: api.Policy{AssistantMode: api.ModeCodeAllowed, ResearchDomains: []string{"docs.python.org"}}})
	ctx := context.Background()
	st, _ := c.StartTurn(ctx, api.AssistantTurnRequest{Message: "research https://docs.python.org/3/library/time.html"})
	evs := drain(t, st)
	if evs[1].Type != api.StreamResearchSource || evs[1].URL != "https://docs.python.org/3/library/time.html" {
		t.Fatalf("allowed: %v", types(evs))
	}
	st, _ = c.StartTurn(ctx, api.AssistantTurnRequest{Message: "research https://stackoverflow.com/questions/1"})
	evs = drain(t, st)
	if evs[1].Type != api.StreamResearchRefused || evs[1].Domain != "stackoverflow.com" {
		t.Fatalf("not allowed: %v", types(evs))
	}
}
