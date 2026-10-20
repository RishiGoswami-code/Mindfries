package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/assistant"
	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/policy"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

// B7: the assistant gateway behind `mindfries agent`. The model runs here
// under the assessment's policy; the CLI carries out the file and command
// tools inside the candidate's workspace; research runs here, on a short list
// of sites. Every prompt, reply, tool call and result is stored as evidence,
// and every model call is counted against the session's token budget.

// AssistantStore is the assistant's storage (implemented by store.DB).
type AssistantStore interface {
	CreateTurn(ctx context.Context, t store.Turn) error
	TurnByID(ctx context.Context, id string) (store.Turn, error)
	SetTurnStatus(ctx context.Context, id, status, errCode string, at time.Time) error
	ClaimTurn(ctx context.Context, id string) (bool, error)
	AddStep(ctx context.Context, turnID, kind, tool string, body json.RawMessage, at time.Time) error
	ConversationSteps(ctx context.Context, sessionID, conversationID string) ([]assistant.Step, error)
	PendingCalls(ctx context.Context, turnID string) ([]string, error)
	ConversationKnown(ctx context.Context, sessionID, conversationID string) (bool, error)
	RecordUsage(ctx context.Context, turnID, sessionID, purpose, model string, in, out int, at time.Time) error
	SessionTokens(ctx context.Context, sessionID string) (int, error)
	TokensSince(ctx context.Context, since time.Time) (int, error)
}

// unlimitedTokens is what a session with no cap reports as its budget.
const unlimitedTokens = 1_000_000_000

const maxRounds = 5 // model calls in one turn (research can loop back)

func (s *Server) registerAssistant(mux *http.ServeMux) {
	if s.sessions == nil || s.assist == nil {
		return
	}
	mux.HandleFunc("POST "+cliPrefix+"/session/assistant/turns", s.assistantTurn)
	mux.HandleFunc("POST "+cliPrefix+"/session/assistant/turns/{id}/tool-results", s.assistantToolResults)
}

// model is the configured model, or a scripted one when there's no key.
func (s *Server) model() assistant.Model {
	if s.assistModel != nil {
		return s.assistModel
	}
	if s.cfg.HasGemini() {
		return assistant.Gemini{APIKey: s.cfg.GeminiKey, Model: s.cfg.GeminiModel, BaseURL: s.cfg.GeminiBaseURL}
	}
	return assistant.Scripted{}
}

func (s *Server) modelName() string {
	if s.assistModel != nil {
		return "test"
	}
	if s.cfg.HasGemini() {
		return s.cfg.GeminiModel
	}
	return "scripted"
}

// budget is what the session has left to spend.
func (s *Server) budget(ctx context.Context, sess session.Session) (total, remaining int) {
	total = s.cfg.MaxTokensPerSession
	if total <= 0 { // no cap configured
		return unlimitedTokens, unlimitedTokens
	}
	if s.assist == nil {
		return total, total
	}
	used, _ := s.assist.SessionTokens(ctx, sess.ID)
	remaining = total - used
	if remaining < 0 {
		remaining = 0
	}
	return total, remaining
}

// sse writes server-sent events in the contract's shape.
type sse struct {
	w  http.ResponseWriter
	fl http.Flusher
}

func newSSE(w http.ResponseWriter) *sse {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	return &sse{w: w, fl: fl}
}

func (e *sse) send(typ string, fields map[string]any) {
	out := map[string]any{"type": typ}
	for k, v := range fields {
		out[k] = v
	}
	b, _ := json.Marshal(out)
	fmt.Fprintf(e.w, "event: %s\ndata: %s\n\n", typ, b)
	if e.fl != nil {
		e.fl.Flush()
	}
}

// gate checks what must hold before the model is called: the session is
// running, the assistant is on, and there's budget left.
func (s *Server) assistantGate(w http.ResponseWriter, r *http.Request, sess session.Session) (policy.Config, bool) {
	cfg := s.configFor(r.Context(), sess)
	if sess.Status != "active" {
		problemWith(w, http.StatusConflict, "session_not_active", "the assistant works only while the clock is running",
			map[string]any{"session_status": sess.Status})
		return cfg, false
	}
	if cfg.AssistantMode == policy.Off || !cfg.AssistantMode.Valid() {
		problem(w, http.StatusForbidden, "assistant_off", "the assistant is off for this assessment")
		return cfg, false
	}
	if _, left := s.budget(r.Context(), sess); left <= 0 {
		problem(w, http.StatusTooManyRequests, "budget_exhausted", "this session's assistant budget is used up")
		return cfg, false
	}
	if cap := s.cfg.MaxTokensPerDay; cap > 0 {
		day := s.now().Truncate(24 * time.Hour)
		if used, err := s.assist.TokensSince(r.Context(), day); err == nil && used >= cap {
			problem(w, http.StatusTooManyRequests, "budget_exhausted", "the assistant is at capacity for today")
			return cfg, false
		}
	}
	return cfg, true
}

type turnRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
	Context        struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	} `json:"context"`
}

func (s *Server) assistantTurn(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
	if !ok {
		return
	}
	var req turnRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" || len(req.Message) > 20000 || len(req.Context.Files) > 20 {
		problem(w, http.StatusBadRequest, "malformed", "message is required (up to 20000 characters, up to 20 attached files)")
		return
	}
	cfg, ok := s.assistantGate(w, r, sess)
	if !ok {
		return
	}
	ctx, now := r.Context(), s.now()
	conv := req.ConversationID
	if conv != "" {
		if known, _ := s.assist.ConversationKnown(ctx, sess.ID, conv); !known {
			problem(w, http.StatusBadRequest, "malformed", "unknown conversation_id")
			return
		}
	} else {
		conv = auth.NewID("conv", 6)
	}
	text := req.Message
	for _, f := range req.Context.Files {
		if len(f.Content) > 64<<10 {
			f.Content = f.Content[:64<<10]
		}
		text += "\n\n--- attached: " + f.Path + " ---\n" + f.Content
	}
	t := store.Turn{ID: auth.NewID("turn", 6), SessionID: sess.ID, ConversationID: conv, Channel: "agent", Prompt: req.Message, StartedAt: now}
	if err := s.assist.CreateTurn(ctx, t); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	_ = s.assist.AddStep(ctx, t.ID, assistant.KindIn, "", assistant.TextBody(text), now)
	s.track(ctx, sess.ID, activity.Item{At: now, Severity: activity.Info, Source: activity.Server, Category: activity.Observed,
		Kind: "assistant.turn", Text: "Asked the assistant.", DedupeKey: "turn:" + t.ID})
	s.runTurn(w, r, sess, cfg, t, true)
}

func (s *Server) assistantToolResults(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authedLive(w, r)
	if !ok {
		return
	}
	var req struct {
		Results []assistant.Result `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil || len(req.Results) == 0 {
		problem(w, http.StatusBadRequest, "malformed", "results are required")
		return
	}
	ctx := r.Context()
	t, err := s.assist.TurnByID(ctx, r.PathValue("id"))
	if err != nil || t.SessionID != sess.ID {
		problem(w, http.StatusNotFound, "not_found", "no such turn")
		return
	}
	pending, err := s.assist.PendingCalls(ctx, t.ID)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if t.Status != "waiting_tools" || len(pending) == 0 {
		problem(w, http.StatusConflict, "turn_not_waiting", "this turn isn't waiting for tool results")
		return
	}
	want := map[string]bool{}
	for _, id := range pending {
		want[id] = true
	}
	got := map[string]bool{}
	for _, res := range req.Results {
		if !want[res.CallID] || got[res.CallID] {
			problem(w, http.StatusConflict, "turn_not_waiting", "a result for a call that isn't pending: "+res.CallID)
			return
		}
		switch res.Status {
		case "ok", "rejected", "refused", "error":
		default:
			problem(w, http.StatusBadRequest, "malformed", "status must be ok, rejected, refused or error")
			return
		}
		got[res.CallID] = true
	}
	if len(got) != len(want) {
		problem(w, http.StatusConflict, "turn_not_waiting", "send one result for every pending call")
		return
	}
	cfg, ok := s.assistantGate(w, r, sess)
	if !ok {
		return
	}
	if claimed, err := s.assist.ClaimTurn(ctx, t.ID); err != nil || !claimed {
		problem(w, http.StatusConflict, "turn_not_waiting", "these results were already received")
		return
	}
	now := s.now()
	for _, res := range req.Results {
		if len(res.Output) > 64<<10 {
			res.Output = res.Output[:64<<10]
		}
		b, _ := json.Marshal(res)
		_ = s.assist.AddStep(ctx, t.ID, assistant.KindResult, "", b, now)
	}
	s.runTurn(w, r, sess, cfg, t, false)
}

// runTurn drives the model until it has an answer or needs the CLI to carry
// out tools, streaming what it does. Research runs here and loops back to the
// model; file and command tools end the stream with tool.calls_pending.
func (s *Server) runTurn(w http.ResponseWriter, r *http.Request, sess session.Session, cfg policy.Config, t store.Turn, first bool) {
	ctx := r.Context()
	out := newSSE(w)
	budgetField := func() map[string]any {
		total, left := s.budget(ctx, sess)
		return map[string]any{"tokens_total": total, "tokens_remaining": left}
	}
	if first {
		out.send("turn.started", map[string]any{"turn_id": t.ID, "conversation_id": t.ConversationID, "mode": string(cfg.AssistantMode), "budget": budgetField()})
	}
	fail := func(code, msg string) {
		_ = s.assist.SetTurnStatus(context.WithoutCancel(ctx), t.ID, "failed", code, s.now())
		out.send("error", map[string]any{"code": code, "message": msg})
	}

	task, _ := s.taskOf(sess)
	system := assistant.System(cfg.AssistantMode, firstNonEmpty(task.Title, sess.AssessmentTitle), sess.Brief, researchDomains(cfg))
	// The researcher's reach is always the assessment's own list; s.researcher
	// only carries how it connects (tests point it at a local server).
	researcher := &assistant.Researcher{}
	if s.researcher != nil {
		cp := *s.researcher
		researcher = &cp
	}
	researcher.Allowed = cfg.ResearchDomains
	tools := assistant.ToolsFor(cfg.AssistantMode)

	for round := 0; round < maxRounds; round++ {
		steps, err := s.assist.ConversationSteps(ctx, sess.ID, t.ConversationID)
		if err != nil {
			fail("internal", "couldn't read the conversation")
			return
		}
		resp, err := s.model().Generate(ctx, assistant.Request{System: system, History: assistant.HistoryFromSteps(steps), Tools: tools})
		if err != nil {
			s.log.Error("assistant: model failed", "session", sess.ID, "err", err)
			if errors.Is(err, assistant.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) {
				fail("model_unavailable", "The assistant's model is busy. Try again in a moment.")
			} else {
				fail("internal", "The assistant couldn't answer.")
			}
			return
		}
		in, outTok := resp.InputTokens, resp.OutputTokens
		if in == 0 && outTok == 0 {
			in, outTok = assistant.Cost(resp.Text), assistant.Cost(resp.Text)
		}
		_ = s.assist.RecordUsage(ctx, t.ID, sess.ID, "agent", s.modelName(), in, outTok, s.now())

		if resp.Text != "" {
			_ = s.assist.AddStep(ctx, t.ID, assistant.KindOut, "", assistant.TextBody(resp.Text), s.now())
			for _, c := range assistant.Chunks(resp.Text, 24) {
				out.send("text.delta", map[string]any{"text": c})
			}
		}

		var forCLI []string
		researched := false
		for _, c := range resp.Calls {
			c.CallID = auth.NewID("call", 6)
			if !assistant.Allowed(cfg.AssistantMode, c.Name) {
				// Policy is enforced here whatever the model asked for.
				msg := "I can't do that in this assessment (" + string(cfg.AssistantMode) + "): " + c.Name + "."
				_ = s.assist.AddStep(ctx, t.ID, assistant.KindOut, "", assistant.TextBody(msg), s.now())
				out.send("text.delta", map[string]any{"text": msg})
				s.track(ctx, sess.ID, activity.Item{At: s.now(), Severity: activity.Notice, Source: activity.Server, Category: activity.Observed,
					Kind: "assistant.refused", Text: "The assistant was stopped from using " + c.Name + " (" + string(cfg.AssistantMode) + " mode).",
					DedupeKey: "refused:" + c.CallID})
				continue
			}
			cb, _ := json.Marshal(c)
			if c.Name == assistant.Research {
				researched = true
				s.doResearch(ctx, out, researcher, sess, t, c, cb)
				continue
			}
			_ = s.assist.AddStep(ctx, t.ID, assistant.KindCall, c.Name, cb, s.now())
			out.send("tool.call", map[string]any{"call": map[string]any{"call_id": c.CallID, "name": c.Name, "input": json.RawMessage(c.Input)}})
			forCLI = append(forCLI, c.CallID)
		}
		if len(forCLI) > 0 {
			_ = s.assist.SetTurnStatus(ctx, t.ID, "waiting_tools", "", s.now())
			out.send("tool.calls_pending", map[string]any{"turn_id": t.ID, "call_ids": forCLI})
			return
		}
		if researched {
			continue // the model reads what it found and answers
		}
		_ = s.assist.SetTurnStatus(ctx, t.ID, "completed", "", s.now())
		out.send("turn.completed", map[string]any{"turn_id": t.ID, "budget": budgetField()})
		return
	}
	_ = s.assist.SetTurnStatus(ctx, t.ID, "completed", "", s.now())
	out.send("turn.completed", map[string]any{"turn_id": t.ID, "budget": budgetField()})
}

// doResearch runs a research call on the server and records both the call and
// what came of it, so the model's next round sees it.
func (s *Server) doResearch(ctx context.Context, out *sse, rs *assistant.Researcher, sess session.Session, t store.Turn, c assistant.Call, callJSON []byte) {
	now := s.now()
	_ = s.assist.AddStep(ctx, t.ID, assistant.KindCall, c.Name, callJSON, now)
	var in struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(c.Input, &in)
	result := assistant.Result{CallID: c.CallID, Status: "ok"}
	title, text, err := rs.Fetch(ctx, in.URL)
	switch {
	case errors.Is(err, assistant.ErrDomain):
		host := assistant.HostOf(in.URL)
		result.Status, result.Output = "refused", "That site isn't an allowed research source for this assessment."
		out.send("research.refused", map[string]any{"domain": host})
		s.track(ctx, sess.ID, activity.Item{At: now, Severity: activity.Notice, Source: activity.Server, Category: activity.Observed,
			Kind: "research.refused", Text: "The assistant was refused " + host + " (not an allowed source).", DedupeKey: "research:" + c.CallID})
	case err != nil:
		result.Status, result.Output = "error", err.Error()
	default:
		result.Output = "# " + title + "\n\n" + text
		out.send("research.source", map[string]any{"url": in.URL, "title": title})
		s.track(ctx, sess.ID, activity.Item{At: now, Severity: activity.Info, Source: activity.Server, Category: activity.Observed,
			Kind: "research.source", Text: "The assistant read " + assistant.HostOf(in.URL) + ".", DedupeKey: "research:" + c.CallID})
	}
	rb, _ := json.Marshal(result)
	_ = s.assist.AddStep(ctx, t.ID, assistant.KindResult, "", rb, s.now())
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
