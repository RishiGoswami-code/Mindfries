package fakeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
)

// The assistant gateway (v2/PLAN.md task 1.14). The model runs here, on the
// server: the CLI never holds a model key. The fake's "model" is a small
// set of scripted rules, enough to exercise every tool and policy path;
// SetModel replaces it in tests.

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /session/assistant/turns", s.authed(s.handleTurn))
		route("POST /session/assistant/turns/{id}/tool-results", s.authed(s.handleToolResults))
	})
}

// Step is one thing the model does in a turn: say something, call a tool,
// or research a URL (handled by the server, never sent to the CLI).
type Step struct {
	Text     string
	Call     *api.ToolCall
	Research string
}

// ModelInput is what the model sees for one step of a turn.
type ModelInput struct {
	Mode    string
	Message string
	// Results are present when the turn continues after tool calls.
	Results []api.ToolResult
}

// Model decides a turn's steps.
type Model func(ModelInput) []Step

// LogEntry is one line of the assistant's evidence log.
type LogEntry struct {
	Kind    string // prompt, reply, tool_call, tool_result, research, refused
	TurnID  string
	Content string
}

type turn struct {
	id, conv string
	pending  map[string]api.ToolCall
	message  string
}

type assistantState struct {
	turns map[string]*turn
	log   []LogEntry
}

// SetModel replaces the scripted model.
func (s *Server) SetModel(m Model) { s.mu.Lock(); s.model = m; s.mu.Unlock() }

// AssistantLog returns a session's assistant evidence log.
func (s *Server) AssistantLog(id string) []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil {
		return append([]LogEntry(nil), ss.ext.assistant.log...)
	}
	return nil
}

func (ss *session) logf(kind, turnID, format string, args ...any) {
	ss.ext.assistant.log = append(ss.ext.assistant.log, LogEntry{Kind: kind, TurnID: turnID, Content: fmt.Sprintf(format, args...)})
}

// cost is the fake's token count: about four characters a token.
func cost(s string) int { return len(s)/4 + 1 }

func (s *Server) gate(w http.ResponseWriter, ss *session) bool {
	if !requireActive(w, ss) {
		return false
	}
	if ss.Policy.AssistantMode == api.ModeOff {
		problem(w, http.StatusForbidden, api.CodeAssistantOff, "The assistant is off for this assessment", "")
		return false
	}
	if b := ss.Policy.AssistantBudget; b != nil && b.TokensRemaining <= 0 {
		problem(w, http.StatusTooManyRequests, api.CodeBudgetExhausted, "This session's assistant budget is used up", "")
		return false
	}
	return true
}

func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request, ss *session) {
	var in api.AssistantTurnRequest
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Message) == "" {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "message is required", "")
		return
	}
	if !s.gate(w, ss) {
		return
	}
	if in.ConversationID == "" {
		in.ConversationID = randomID("conv_", 6)
	}
	t := &turn{id: randomID("turn_", 6), conv: in.ConversationID, pending: map[string]api.ToolCall{}, message: in.Message}
	ss.ext.assistant.turns[t.id] = t
	ss.logf("prompt", t.id, "%s", in.Message)
	s.charge(ss, in.Message)
	steps := s.modelFor()(ModelInput{Mode: ss.Policy.AssistantMode, Message: in.Message})
	s.stream(w, ss, t, steps, true)
}

func (s *Server) handleToolResults(w http.ResponseWriter, r *http.Request, ss *session) {
	var in api.ToolResults
	if !decode(w, r, &in) {
		return
	}
	t := ss.ext.assistant.turns[r.PathValue("id")]
	if t == nil {
		problem(w, http.StatusNotFound, api.CodeNotFound, "No such turn", "")
		return
	}
	if len(t.pending) == 0 {
		problem(w, http.StatusConflict, api.CodeTurnNotWaiting, "This turn isn't waiting for tool results", "")
		return
	}
	for _, res := range in.Results {
		if _, ok := t.pending[res.CallID]; !ok {
			problem(w, http.StatusConflict, api.CodeTurnNotWaiting, "Result for a call that isn't pending", res.CallID)
			return
		}
	}
	if !s.gate(w, ss) {
		return
	}
	for _, res := range in.Results {
		delete(t.pending, res.CallID)
		b, _ := json.Marshal(res)
		ss.logf("tool_result", t.id, "%s", b)
		s.charge(ss, res.Output)
	}
	steps := s.modelFor()(ModelInput{Mode: ss.Policy.AssistantMode, Message: t.message, Results: in.Results})
	s.stream(w, ss, t, steps, false)
}

func (s *Server) charge(ss *session, text string) {
	if b := ss.Policy.AssistantBudget; b != nil {
		b.TokensRemaining -= cost(text)
		if b.TokensRemaining < 0 {
			b.TokensRemaining = 0
		}
	}
}

func (s *Server) modelFor() Model {
	if s.model != nil {
		return s.model
	}
	return scriptedModel
}

// stream writes the turn's events. Policy is enforced here, whatever the
// model asked for: in explain_only mode no edit or command reaches the CLI.
func (s *Server) stream(w http.ResponseWriter, ss *session, t *turn, steps []Step, first bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	send := func(ev api.StreamEvent) {
		b, _ := json.Marshal(ev)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	budget := func() *api.Budget {
		if b := ss.Policy.AssistantBudget; b != nil {
			c := *b
			return &c
		}
		return nil
	}
	if first {
		send(api.StreamEvent{Type: api.StreamTurnStarted, TurnID: t.id, ConversationID: t.conv, Mode: ss.Policy.AssistantMode, Budget: budget()})
	}
	var calls []string
	for _, st := range steps {
		switch {
		case st.Text != "":
			ss.logf("reply", t.id, "%s", st.Text)
			s.charge(ss, st.Text)
			for _, chunk := range chunks(st.Text, 24) {
				send(api.StreamEvent{Type: api.StreamTextDelta, Text: chunk})
			}
		case st.Research != "":
			u, err := url.Parse(st.Research)
			host := ""
			if err == nil {
				host = strings.ToLower(u.Hostname())
			}
			if !allowedDomain(host, ss.Policy.ResearchDomains) {
				ss.logf("refused", t.id, "research %s", st.Research)
				send(api.StreamEvent{Type: api.StreamResearchRefused, Domain: host})
				continue
			}
			ss.logf("research", t.id, "%s", st.Research)
			send(api.StreamEvent{Type: api.StreamResearchSource, URL: st.Research, Title: "Documentation: " + host})
		case st.Call != nil:
			if ss.Policy.AssistantMode != api.ModeCodeAllowed && (st.Call.Name == api.ToolProposeEdit || st.Call.Name == api.ToolRunCommand) {
				ss.logf("refused", t.id, "%s in %s mode", st.Call.Name, ss.Policy.AssistantMode)
				msg := "In this assessment I can explain and research, but I can't write code or run commands. I can walk you through it instead."
				ss.logf("reply", t.id, "%s", msg)
				send(api.StreamEvent{Type: api.StreamTextDelta, Text: msg})
				continue
			}
			c := *st.Call
			if c.CallID == "" {
				c.CallID = randomID("call_", 6)
			}
			t.pending[c.CallID] = c
			b, _ := json.Marshal(c)
			ss.logf("tool_call", t.id, "%s", b)
			send(api.StreamEvent{Type: api.StreamToolCall, Call: &c})
			calls = append(calls, c.CallID)
		}
	}
	if len(calls) > 0 {
		send(api.StreamEvent{Type: api.StreamCallsPending, TurnID: t.id, CallIDs: calls})
		return
	}
	send(api.StreamEvent{Type: api.StreamTurnCompleted, TurnID: t.id, Budget: budget()})
}

func chunks(s string, n int) []string {
	var out []string
	for len(s) > n {
		out = append(out, s[:n])
		s = s[n:]
	}
	return append(out, s)
}

// defaultResearch is used when the assessment lists no domains.
var defaultResearch = []string{"docs.python.org", "developer.mozilla.org", "pkg.go.dev", "nodejs.org", "pypi.org", "npmjs.com"}

func allowedDomain(host string, allowed []string) bool {
	if host == "" {
		return false
	}
	if len(allowed) == 0 {
		allowed = defaultResearch
	}
	for _, a := range allowed {
		a = strings.ToLower(a)
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

var (
	reRead     = regexp.MustCompile(`(?i)^read (\S+)`)
	reList     = regexp.MustCompile(`(?i)^list( files)?\b(?: (\S+))?`)
	reCreate   = regexp.MustCompile(`(?is)^(?:create|write) (\S+)(?: with|:)\s?(.*)$`)
	reReplace  = regexp.MustCompile(`(?is)^replace (.+?) with (.+?) in (\S+)$`)
	reRun      = regexp.MustCompile(`(?i)^run (.+)$`)
	reResearch = regexp.MustCompile(`(?i)^(?:research|look up) (\S+)`)
)

func call(name string, input any) *api.ToolCall {
	b, _ := json.Marshal(input)
	return &api.ToolCall{Name: name, Input: b}
}

// scriptedModel understands a few literal requests, which the tests and
// manual trials use: "read <path>", "list files", "create <path> with
// <text>", "replace <old> with <new> in <path>", "run <command>",
// "research <url>". Anything else gets a generic answer.
func scriptedModel(in ModelInput) []Step {
	if in.Results != nil {
		var parts []string
		for _, r := range in.Results {
			switch {
			case r.Status == api.ToolRejected:
				parts = append(parts, "OK, I won't make that change.")
			case r.Status == api.ToolRefused:
				parts = append(parts, "That isn't allowed here: "+r.Output)
			case r.AppliedEdit != nil:
				parts = append(parts, fmt.Sprintf("Done: %s is updated.", r.AppliedEdit.Path))
			case r.ExitCode != nil:
				parts = append(parts, fmt.Sprintf("The command finished with exit code %d.", *r.ExitCode))
			default:
				parts = append(parts, fmt.Sprintf("I've read it (%d characters).", len(r.Output)))
			}
		}
		return []Step{{Text: strings.Join(parts, " ")}}
	}
	m := strings.TrimSpace(in.Message)
	switch {
	case reRead.MatchString(m):
		return []Step{{Call: call(api.ToolReadFile, api.ReadFileInput{Path: reRead.FindStringSubmatch(m)[1]})}}
	case reList.MatchString(m):
		return []Step{{Call: call(api.ToolListFiles, api.ListFilesInput{Dir: reList.FindStringSubmatch(m)[2]})}}
	case reCreate.MatchString(m):
		g := reCreate.FindStringSubmatch(m)
		return []Step{{Text: "Here's a version you can apply."}, {Call: call(api.ToolProposeEdit, api.ProposeEditInput{Path: g[1], Create: true, Edits: []api.TextEdit{{OldText: "", NewText: g[2]}}})}}
	case reReplace.MatchString(m):
		g := reReplace.FindStringSubmatch(m)
		return []Step{{Call: call(api.ToolProposeEdit, api.ProposeEditInput{Path: g[3], Edits: []api.TextEdit{{OldText: g[1], NewText: g[2]}}})}}
	case reRun.MatchString(m):
		return []Step{{Call: call(api.ToolRunCommand, api.RunCommandInput{Command: reRun.FindStringSubmatch(m)[1]})}}
	case reResearch.MatchString(m):
		return []Step{{Research: reResearch.FindStringSubmatch(m)[1]}, {Text: "Here's what the documentation says."}}
	}
	return []Step{{Text: "Let's think it through. What have you tried so far, and what did you expect to happen?"}}
}
