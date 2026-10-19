// Package assistant is `mindfries agent`: the coding and research assistant
// in the candidate's terminal (v2/PLAN.md Stage 1b). The model runs on the
// server under the assessment's policy; this side shows replies as they
// stream, carries out tool calls inside the task folder with the
// candidate's approval, and records every edit it applies.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/ui"
)

// Chat is one `mindfries agent` session in the terminal.
type Chat struct {
	Client *api.Client
	Exec   *Executor
	// View shows everything; nil means the plain view on UI and Out.
	View   View
	UI     *ui.UI
	Out    io.Writer
	Policy api.Policy
	// Title and Workspace are shown when the agent opens.
	Title     string
	Workspace string
	// ConvPath remembers the conversation between runs.
	ConvPath string
	// NewConversation starts afresh instead of carrying on.
	NewConversation bool

	conv   string
	budget *api.Budget
}

var modeText = map[string]string{
	api.ModeExplainOnly: "explain only: it explains and researches, but won't write code or run commands",
	api.ModeCodeAllowed: "it can propose code changes and commands; nothing happens without your yes",
}

// Commands are the slash commands, for /help and the TUI's hints.
var Commands = [][2]string{
	{"/help", "show the commands"},
	{"/new", "start a new conversation"},
	{"/budget", "show the tokens left"},
	{"/clear", "clear the screen"},
	{"/exit", "leave (your session carries on)"},
}

func (c *Chat) view() View {
	if c.View == nil {
		c.View = &Plain{UI: c.UI, Out: c.Out}
	}
	return c.View
}

// Run reads the candidate's messages until /exit or end of input.
func (c *Chat) Run(ctx context.Context) error {
	v := c.view()
	if c.Exec != nil && c.Exec.View == nil {
		c.Exec.View = v
	}
	if c.Policy.AssistantMode == api.ModeOff {
		v.Notice("The assistant is off for this assessment.")
		return nil
	}
	c.budget = c.Policy.AssistantBudget
	if !c.NewConversation {
		c.loadConv()
	}
	v.Welcome(Welcome{Title: c.Title, Workspace: c.Workspace, Mode: modeText[c.Policy.AssistantMode], Budget: c.budget})
	for {
		msg, ok := v.Read()
		if !ok {
			return nil
		}
		switch strings.ToLower(strings.TrimSpace(msg)) {
		case "":
			continue
		case "/exit", "/quit", "exit", "quit":
			return nil
		case "/help", "help":
			var lines []string
			for _, cmd := range Commands {
				lines = append(lines, fmt.Sprintf("%-8s %s", cmd[0], cmd[1]))
			}
			v.Help(lines)
			continue
		case "/new":
			c.conv = ""
			c.saveConv()
			v.Notice("Started a new conversation.")
			continue
		case "/budget":
			c.printBudget()
			continue
		case "/clear":
			v.Clear()
			continue
		}
		if err := c.turn(ctx, msg); err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() == nil {
				v.TurnFailed("Interrupted. What should it do instead?")
				continue
			}
			if p, ok := api.AsProblem(err); ok {
				switch p.Code {
				case api.CodeBudgetExhausted:
					v.Notice("The assistant's budget for this session is used up. You can keep working without it.")
					continue
				case api.CodeAssistantOff:
					v.Notice("The assistant is off for this assessment.")
					return nil
				}
				if api.IsAuthFailure(err) || p.Code == api.CodeSessionNotActive {
					return errors.New("this assessment session has ended")
				}
			}
			v.TurnFailed(fmt.Sprintf("That didn't go through (%v). Try again.", err))
		}
	}
}

func (c *Chat) printBudget() {
	if c.budget == nil {
		c.view().Notice("No budget is set for this session.")
		return
	}
	c.view().Notice(fmt.Sprintf("%d of %d tokens left.", c.budget.TokensRemaining, c.budget.TokensTotal))
}

// turn sends one message and follows the turn through any tool calls. Esc
// (in the TUI) cancels the reply while it streams.
func (c *Chat) turn(ctx context.Context, msg string) error {
	v := c.view()
	tctx, cancel := context.WithCancel(ctx)
	defer cancel()
	v.TurnStart()
	stopWatch := v.Interruptible(cancel)
	st, err := c.Client.StartTurn(tctx, api.AssistantTurnRequest{ConversationID: c.conv, Message: msg})
	if err != nil {
		stopWatch()
		return err
	}
	for {
		turnID, calls, err := c.follow(st)
		st.Close()
		stopWatch()
		if err != nil {
			if tctx.Err() != nil {
				return context.Canceled
			}
			return err
		}
		if len(calls) == 0 {
			return nil
		}
		var results []api.ToolResult
		for _, call := range calls {
			results = append(results, c.Exec.Execute(ctx, call))
		}
		v.ResultsSent()
		stopWatch = v.Interruptible(cancel)
		st, err = c.Client.SendToolResults(tctx, turnID, api.ToolResults{Results: results})
		if err != nil {
			stopWatch()
			if tctx.Err() != nil {
				return context.Canceled
			}
			return err
		}
	}
}

// follow shows a stream and returns the calls it asked for, if any.
func (c *Chat) follow(st *api.Stream) (string, []api.ToolCall, error) {
	v := c.view()
	var calls []api.ToolCall
	for {
		ev, err := st.Next()
		if err == io.EOF {
			return "", nil, errors.New("the reply was cut off")
		}
		if err != nil {
			return "", nil, err
		}
		switch ev.Type {
		case api.StreamTurnStarted:
			if ev.ConversationID != "" && ev.ConversationID != c.conv {
				c.conv = ev.ConversationID
				c.saveConv()
			}
			if ev.Budget != nil {
				c.budget = ev.Budget
			}
		case api.StreamTextDelta:
			v.Text(ev.Text)
		case api.StreamResearchSource:
			title := ev.Title
			if title == "" {
				title = ev.URL
			}
			v.Source(title, ev.URL)
		case api.StreamResearchRefused:
			v.ResearchRefused(ev.Domain)
		case api.StreamToolCall:
			if ev.Call != nil {
				calls = append(calls, *ev.Call)
			}
		case api.StreamCallsPending:
			v.CallsPending()
			return ev.TurnID, calls, nil
		case api.StreamTurnCompleted:
			if ev.Budget != nil {
				c.budget = ev.Budget
			}
			v.TurnDone(ev.Budget)
			return ev.TurnID, nil, nil
		case api.StreamError:
			if ev.Code == api.CodeBudgetExhausted {
				return "", nil, &api.Problem{Code: api.CodeBudgetExhausted, Title: ev.Message, Status: 429}
			}
			return "", nil, fmt.Errorf("the assistant stopped: %s", orText(ev.Message, ev.Code))
		}
	}
}

func orText(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type convFile struct {
	ConversationID string `json:"conversation_id"`
}

func (c *Chat) loadConv() {
	b, err := os.ReadFile(c.ConvPath)
	if err != nil {
		return
	}
	var f convFile
	if json.Unmarshal(b, &f) == nil {
		c.conv = f.ConversationID
	}
}

func (c *Chat) saveConv() {
	if c.ConvPath == "" {
		return
	}
	b, _ := json.Marshal(convFile{ConversationID: c.conv})
	_ = state.WriteFileAtomic(c.ConvPath, b, 0o600)
}
