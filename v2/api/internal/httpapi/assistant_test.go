package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mindfries/v2/api/internal/assistant"
	"github.com/mindfries/v2/api/internal/config"
	"github.com/mindfries/v2/api/internal/policy"
)

type streamed struct {
	Type string
	Data map[string]any
}

// parseStream reads a text/event-stream body into events.
func parseStream(t *testing.T, raw []byte) []streamed {
	t.Helper()
	var out []streamed
	for _, block := range strings.Split(string(raw), "\n\n") {
		var typ, data string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event: ") {
				typ = line[7:]
			}
			if strings.HasPrefix(line, "data: ") {
				data = line[6:]
			}
		}
		if typ == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatalf("bad event data %q: %v", data, err)
		}
		out = append(out, streamed{typ, m})
	}
	return out
}

func types(evs []streamed) string {
	var ts []string
	for _, e := range evs {
		ts = append(ts, e.Type)
	}
	return strings.Join(ts, ",")
}

func textOf(evs []streamed) string {
	var b strings.Builder
	for _, e := range evs {
		if e.Type == "text.delta" {
			b.WriteString(e.Data["text"].(string))
		}
	}
	return b.String()
}

// assistantEnv is an env with an assistant mode configured, and an active session.
func assistantEnv(t *testing.T, mode policy.AssistantMode, mod func(*config.Config), opts func(*Options)) (*env, map[string]any, string, string) {
	e := newEnvWith(t, mod, opts)
	g := newGrant()
	c := policy.Default()
	c.AssessmentID = g["assessment_id"].(string)
	c.AssistantMode = mode
	if err := e.db.SaveAssessmentConfig(t.Context(), c, "test"); err != nil {
		t.Fatal(err)
	}
	cli, sid := e.startSession(g)
	panel := e.panelToken(g, sid)
	e.shareAll(panel)
	if r := e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"}); r.Status != 200 {
		t.Fatalf("begin: %d %s", r.Status, r.Raw)
	}
	return e, g, cli, sid
}

func (e *env) ask(cli, message, conv string) reply {
	body := map[string]any{"message": message}
	if conv != "" {
		body["conversation_id"] = conv
	}
	return e.cli("POST", "/session/assistant/turns", cli, body)
}

func TestExplainOnlyAnswersAndRemembersTheConversation(t *testing.T) {
	e, _, cli, sid := assistantEnv(t, policy.ExplainOnly, nil, nil)
	r := e.ask(cli, "why are all the tickets going to ana?", "")
	if r.Status != 200 || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	evs := parseStream(t, r.Raw)
	if !strings.HasPrefix(types(evs), "turn.started,text.delta") || !strings.HasSuffix(types(evs), "turn.completed") {
		t.Fatalf("%s", types(evs))
	}
	if !strings.Contains(textOf(evs), "What have you tried") {
		t.Fatalf("%q", textOf(evs))
	}
	start := evs[0].Data
	conv, turn := start["conversation_id"].(string), start["turn_id"].(string)
	b0 := start["budget"].(map[string]any)
	if start["mode"] != "explain_only" || b0["tokens_total"].(float64) <= 0 {
		t.Fatalf("%v", start)
	}
	end := evs[len(evs)-1].Data
	if end["turn_id"] != turn || end["budget"].(map[string]any)["tokens_remaining"].(float64) >= b0["tokens_remaining"].(float64) {
		t.Fatalf("a turn costs budget: %v vs %v", end, b0)
	}

	// The conversation carries on, and everything is stored as evidence.
	r2 := e.ask(cli, "I tried reading routing.go", conv)
	if parseStream(t, r2.Raw)[0].Data["conversation_id"] != conv {
		t.Fatal("same conversation")
	}
	steps, _ := e.db.ConversationSteps(t.Context(), sid, conv)
	if len(steps) != 4 || steps[0].Kind != "transcript_in" || steps[1].Kind != "transcript_out" {
		t.Fatalf("steps: %+v", steps)
	}
	if r := e.ask(cli, "hi", "conv_nope"); r.Status != http.StatusBadRequest {
		t.Fatalf("an unknown conversation: %d", r.Status)
	}
	if r := e.ask(cli, "   ", ""); r.Status != http.StatusBadRequest {
		t.Fatalf("an empty message: %d", r.Status)
	}
	// The tracker notes that the assistant was used (without the content).
	items := e.panel("GET", "/v2/panel/activity", e.panelToken(map[string]any{"candidate_id": candidateOf(e, sid)}, sid), nil).JSON()["items"].([]any)
	found := false
	for _, it := range items {
		if it.(map[string]any)["text"] == "Asked the assistant." {
			found = true
		}
		if strings.Contains(it.(map[string]any)["text"].(string), "ana") {
			t.Error("the tracker must not carry what the candidate typed")
		}
	}
	if !found {
		t.Fatal("the tracker should note an assistant turn")
	}
}

func candidateOf(e *env, sessionID string) string {
	s, _ := e.db.SessionByID(context.Background(), sessionID)
	return s.Links.CandidateID
}

func TestExplainOnlyNeverEditsOrRunsEvenIfTheModelAsks(t *testing.T) {
	// A model that ignores its instructions and asks to edit and run.
	rogue := modelFunc(func(r assistant.Request) assistant.Response {
		return assistant.Response{Text: "Sure.", Calls: []assistant.Call{
			{Name: assistant.ProposeEdit, Input: json.RawMessage(`{"path":"a.go","edits":[{"old_text":"a","new_text":"b"}]}`)},
			{Name: assistant.RunCommand, Input: json.RawMessage(`{"command":"rm -rf /"}`)},
		}}
	})
	e, _, cli, _ := assistantEnv(t, policy.ExplainOnly, nil, func(o *Options) { o.Model = rogue })
	evs := parseStream(t, e.ask(cli, "fix it for me", "").Raw)
	if strings.Contains(types(evs), "tool.call") || strings.Contains(types(evs), "tool.calls_pending") {
		t.Fatalf("explain_only must never reach the CLI with an edit or a command: %s", types(evs))
	}
	if !strings.Contains(textOf(evs), "can't do that in this assessment") || !strings.HasSuffix(types(evs), "turn.completed") {
		t.Fatalf("%q %s", textOf(evs), types(evs))
	}
}

type modelFunc func(assistant.Request) assistant.Response

func (f modelFunc) Generate(_ context.Context, r assistant.Request) (assistant.Response, error) {
	return f(r), nil
}

func TestCodeAllowedRunsToolsThroughTheCLI(t *testing.T) {
	e, _, cli, _ := assistantEnv(t, policy.CodeAllowed, nil, nil)
	evs := parseStream(t, e.ask(cli, "read internal/support/routing.go", "").Raw)
	if types(evs) != "turn.started,tool.call,tool.calls_pending" {
		t.Fatalf("%s", types(evs))
	}
	call := evs[1].Data["call"].(map[string]any)
	if call["name"] != "read_file" || call["input"].(map[string]any)["path"] != "internal/support/routing.go" {
		t.Fatalf("%v", call)
	}
	turn, callID := evs[0].Data["turn_id"].(string), call["call_id"].(string)
	if ids := evs[2].Data["call_ids"].([]any); len(ids) != 1 || ids[0] != callID {
		t.Fatalf("%v", evs[2].Data)
	}
	post := func(results any) reply {
		return e.cli("POST", "/session/assistant/turns/"+turn+"/tool-results", cli, map[string]any{"results": results})
	}
	// Wrong call, wrong status, missing result: refused, and the turn is still waiting.
	if r := post([]map[string]any{{"call_id": "call_other", "status": "ok"}}); r.Status != http.StatusConflict || r.str("code") != "turn_not_waiting" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := post([]map[string]any{{"call_id": callID, "status": "great"}}); r.Status != http.StatusBadRequest {
		t.Fatalf("%d", r.Status)
	}
	// The real result continues the turn on a fresh stream.
	r := post([]map[string]any{{"call_id": callID, "status": "ok", "output": "package support\n// routing"}})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	evs = parseStream(t, r.Raw)
	if !strings.HasSuffix(types(evs), "turn.completed") || !strings.Contains(textOf(evs), "Got it") {
		t.Fatalf("%s %q", types(evs), textOf(evs))
	}
	// Results are one-shot.
	if r := post([]map[string]any{{"call_id": callID, "status": "ok"}}); r.Status != http.StatusConflict || r.str("code") != "turn_not_waiting" {
		t.Fatalf("a second post: %d %s", r.Status, r.Raw)
	}
	// Someone else's turn is not found.
	e2, _, cli2, _ := assistantEnv(t, policy.CodeAllowed, nil, nil)
	if r := e2.cli("POST", "/session/assistant/turns/"+turn+"/tool-results", cli2, map[string]any{"results": []map[string]any{{"call_id": callID, "status": "ok"}}}); r.Status != http.StatusNotFound {
		t.Fatalf("%d", r.Status)
	}
}

func TestAnAcceptedEditIsRecordedExactly(t *testing.T) {
	e, _, cli, sid := assistantEnv(t, policy.CodeAllowed, nil, nil)
	evs := parseStream(t, e.ask(cli, "replace x with y in a.go", "").Raw)
	turn, conv := evs[0].Data["turn_id"].(string), evs[0].Data["conversation_id"].(string)
	callID := evs[1].Data["call"].(map[string]any)["call_id"].(string)
	edit := map[string]any{"edit_id": "edit_1", "path": "a.go", "before_sha256": strings.Repeat("a", 64), "after_sha256": strings.Repeat("b", 64), "ranges": []map[string]any{{"start": 3, "end": 5}}}
	r := e.cli("POST", "/session/assistant/turns/"+turn+"/tool-results", cli, map[string]any{"results": []map[string]any{{"call_id": callID, "status": "ok", "applied_edit": edit}}})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	steps, _ := e.db.ConversationSteps(t.Context(), sid, conv)
	var got string
	for _, st := range steps {
		if st.Kind == "tool_result" {
			got = string(st.Body)
		}
	}
	var rec struct {
		AppliedEdit struct {
			EditID string `json:"edit_id"`
			After  string `json:"after_sha256"`
		} `json:"applied_edit"`
	}
	_ = json.Unmarshal([]byte(got), &rec)
	if rec.AppliedEdit.EditID != "edit_1" || rec.AppliedEdit.After != strings.Repeat("b", 64) {
		t.Fatalf("the applied edit is the record attribution is checked against: %s", got)
	}
}

func TestTheAssistantIsOffWhenTheAssessmentSaysSo(t *testing.T) {
	e, _, cli, _ := assistantEnv(t, policy.Off, nil, nil)
	if r := e.ask(cli, "help", ""); r.Status != http.StatusForbidden || r.str("code") != "assistant_off" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if got := e.cli("GET", "/session", cli, nil).JSON()["policy"].(map[string]any); got["assistant_mode"] != "off" || got["assistant_budget"] != nil {
		t.Fatalf("%v", got)
	}
}

func TestTheAssistantOnlyWorksWhileTheClockRuns(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, _ := e.startSession(g) // ready, never begun
	if r := e.ask(cli, "help", ""); r.Status != http.StatusConflict || r.str("code") != "session_not_active" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
}

func TestBudgetRunsOut(t *testing.T) {
	e, _, cli, _ := assistantEnv(t, policy.ExplainOnly, func(c *config.Config) { c.MaxTokensPerSession = 60 }, nil)
	first := e.ask(cli, strings.Repeat("a long question ", 20), "")
	if first.Status != 200 {
		t.Fatalf("%d %s", first.Status, first.Raw)
	}
	left := parseStream(t, first.Raw)
	if rem := left[len(left)-1].Data["budget"].(map[string]any)["tokens_remaining"].(float64); rem != 0 {
		t.Fatalf("a small budget is spent by one turn: %v", rem)
	}
	if r := e.ask(cli, "one more", ""); r.Status != http.StatusTooManyRequests || r.str("code") != "budget_exhausted" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if b := e.cli("GET", "/session", cli, nil).JSON()["policy"].(map[string]any)["assistant_budget"].(map[string]any); b["tokens_remaining"] != float64(0) {
		t.Fatalf("the session shows its budget: %v", b)
	}
}

func TestResearchRunsOnTheServerAndStaysOnTheList(t *testing.T) {
	docs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><title>Slices</title><body><p>A slice is a view of an array.</p></body></html>`))
	}))
	defer docs.Close()
	host := strings.TrimPrefix(docs.URL, "http://")
	host = host[:strings.LastIndex(host, ":")]

	e, g, cli, sid := assistantEnv(t, policy.ExplainOnly, nil, func(o *Options) {
		o.Researcher = &assistant.Researcher{AllowPrivate: true} // the allowed list comes from the assessment
	})
	c := policy.Default()
	c.AssessmentID = g["assessment_id"].(string)
	c.AssistantMode = policy.ExplainOnly
	c.ResearchDomains = []string{host}
	if err := e.db.SaveAssessmentConfig(t.Context(), c, "test"); err != nil {
		t.Fatal(err)
	}
	// The session tells the CLI what research may reach.
	if d := e.cli("GET", "/session", cli, nil).JSON()["policy"].(map[string]any)["research_domains"].([]any); len(d) != 1 || d[0] != host {
		t.Fatalf("%v", d)
	}
	evs := parseStream(t, e.ask(cli, "research "+docs.URL+"/slices", "").Raw)
	if types(evs) != "turn.started,research.source,text.delta,turn.completed" {
		t.Fatalf("%s", types(evs))
	}
	if evs[1].Data["title"] != "Slices" || evs[1].Data["url"] != docs.URL+"/slices" {
		t.Fatalf("%v", evs[1].Data)
	}
	// Off the list: refused, said so, and no page was fetched.
	evs = parseStream(t, e.ask(cli, "research https://evil.example/x", "").Raw)
	if !strings.Contains(types(evs), "research.refused") || evs[1].Data["domain"] != "evil.example" {
		t.Fatalf("%s %v", types(evs), evs[1].Data)
	}
	items := e.panel("GET", "/v2/panel/activity", e.panelToken(map[string]any{"candidate_id": candidateOf(e, sid)}, sid), nil).JSON()["items"].([]any)
	var texts []string
	for _, it := range items {
		texts = append(texts, it.(map[string]any)["text"].(string))
	}
	all := strings.Join(texts, "\n")
	if !strings.Contains(all, "The assistant read "+host) || !strings.Contains(all, "The assistant was refused evil.example") {
		t.Fatalf("research is in the tracker:\n%s", all)
	}
}

type failingModel struct{ err error }

func (f failingModel) Generate(context.Context, assistant.Request) (assistant.Response, error) {
	return assistant.Response{}, f.err
}

func TestAModelThatIsDownIsSaidPlainly(t *testing.T) {
	e, _, cli, _ := assistantEnv(t, policy.ExplainOnly, nil, func(o *Options) { o.Model = failingModel{assistant.ErrUnavailable} })
	evs := parseStream(t, e.ask(cli, "hello", "").Raw)
	last := evs[len(evs)-1]
	if last.Type != "error" || last.Data["code"] != "model_unavailable" || last.Data["message"] == "" {
		t.Fatalf("%s %v", types(evs), last.Data)
	}
	// And a failure doesn't leave a turn waiting: the next question works.
	e2, _, cli2, _ := assistantEnv(t, policy.ExplainOnly, nil, func(o *Options) { o.Model = failingModel{assistant.ErrUnavailable} })
	if r := e2.ask(cli2, "hello", ""); r.Status != 200 {
		t.Fatalf("%d", r.Status)
	}
}
